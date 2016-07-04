// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package signing signs syncbase updates using public key signatures, and
// allows these signatures to be checked on other nodes.
//
// The functionality is geared specifically towards syncbase synchronization
// because it is designed to allow a signature to remain valid during its
// propagation across the syncgroup once it has been accepted by at least one
// member of a syncgroup, even if the original key or its blessings are
// invalidated in the meantime.
//
// There are three types of participant:
// - an "author", which creates an update, and signs it with Sign().
// - one or more "validators", each of which receives a change directly from
//   the author, and applies Check() to validate it.
// - zero or more "checkers', each of whom receives a change from a validator
//   or another checker, and applied Check() to check it.
//
// A validator checks the signature and blessings provided by the author, and
// then appends its own signature, vouching for the fact that the author's
// signature was good at the time the validator saw it.
//
// A checker checks the signatures of both the author and validator but uses
// weaker checks for signature validity than a validator.  In particular, it
// uses a significant grace period for key expiry so that a change admitted to
// the syncgroup by a validator has an opportunity to propagate to all the
// nodes in the syncgroup if the keys or blessings are revoked after the change
// is admitted, but before it is fully propagated.  The intent is that the
// grace period be chosen to be greater than the diameter of the syncgroup
// (measured in time).  One way to ensure that is to insist that members sync
// with a central server at least every T time units, and make the grace period
// be 2T.  The central server may sign the data anew to allow new members to pick
// it up.
//
// The model is further complicated by performance concerns.  An update written
// to syncbase might be quite small (perhaps tens of bytes) but:
// a) a public key signature or verification can take on the order of a
//    millisecond.  (Currently, ECDSA signing might a little under 1ms and
//    verification just over 2ms on a workstation.  A checker performs two such
//    verifications.)
// b) unmarshalling even a simple Blessings object can take milliseconds. (!)
// c) marshalling a public key can take 10us.
// d) a Blessings object is of the order of a kilobyte of more, which may
//    represent substantial space overhead if duplicated.
//
// Because of (a), we wish to batch syncbase updates, so that a single
// signature check applies to several updates.  Thus the Data in a
// DataWithSignature is a vector of Item, rather than a single Item.
//
// However, we will not always wish to put all updates in the same batch.  For
// example, an author and a validator might share two different syncgroups with
// different memberships.  In such a case, the author might keep the batches
// for one syncgoup separate from batches for the other syncgroup, even though
// the author blessings and validator identities are the same for all the
// batches.  Thus, because of (b,c,d), it's worth decoupling the author's
// Blessings data and the validator's key data separately from the signed
// batches itself, so that the blessings and validator data can be processed
// once, even though several batches of updates are being sent.  A
// ValidationCache is used to hold this data separately, and allow it to be
// sent just once, rather than once per signature.
//
// Lastly, imagine that the author sends a batch of 10 updates to a validator,
// and the validator then syncs with a checker that is permitted to see only
// half of the updates; perhaps ACLs prevent if from seeing the others.  This
// requires that the signature on the batch remain valid even if some of the
// updates in the batch are removed.  This is accomplished via the Item type,
// which is a VDL union type that contains either the bytes of the marshalled
// form of the update, or (if the update must not be sent) the SHA-256 hash of
// the data (which can be computed with SumByteVectorWithLength()).
package signing

import "bytes"
import "crypto/sha256"
import "encoding/binary"
import "hash"
import "time"

import "v.io/x/ref/services/syncbase/signing/krl"
import "v.io/v23/context"
import "v.io/v23/security"
import "v.io/v23/verror"

const pkgPath = "v.io/x/ref/services/syncbase/signing"

// These are among the errors may be returned by Check(), and indicate that the
// operation should be retried when new data has been added to the
// ValidationCache.  The errors are public to make it easier for the client to
// test for them.
var (
	ErrNeedAuthorBlessingsAndValidatorDataForHash = verror.Register(
		pkgPath+".ErrNeedAuthorBlessingsAndValidatorDataForHash",
		verror.RetryRefetch,
		"{1:}{2:} The ValidationCache contains neither the author blessings nor the validator data{:_}")
	ErrNeedAuthorBlessingsForHash = verror.Register(
		pkgPath+".ErrNeedAuthorBlessingsForHash",
		verror.RetryRefetch,
		"{1:}{2:} The ValidationCache does not contain the author blessings{:_}")
	ErrNeedValidatorDataForHash = verror.Register(
		pkgPath+".ErrNeedValidatorDataForHash",
		verror.RetryRefetch,
		"{1:}{2:} The ValidationCache does not contain the validator data{:_}")
)

// These errors are less likely to be tested for, and so are not exported.
var (
	errAuthorKeyIsRevoked = verror.Register(
		pkgPath+".errAuthorKeyIsRevoked",
		verror.NoRetry,
		"{1:}{2:} The author key has been revoked{:_}")
	errBadAuthorSignature = verror.Register(
		pkgPath+".errBadAuthorSignature",
		verror.NoRetry,
		"{1:}{2:} Author signature verification failed{:_}")
	errBadValidatorSignature = verror.Register(
		pkgPath+".errBadValidatorSignature",
		verror.NoRetry,
		"{1:}{2:} Validator signature verification failed{:_}")
	errAuthorBlessingsHaveNoValidNames = verror.Register(
		pkgPath+".errAuthorBlessingsHaveNoValidNames",
		verror.NoRetry,
		"{1:}{2:} Author Blessings have no valid names{:_}")
	errMayNotValidateOwnSignature = verror.Register(
		pkgPath+".errMayNotValidateOwnSignature",
		verror.NoRetry,
		"{1:}{2:} Author may not validate its own signature{:_}")
	errSenderIsNotAuthor = verror.Register(
		pkgPath+".errSenderIsNotAuthor",
		verror.NoRetry,
		"{1:}{2:} Author is not sender of RPC; will not validate{:_}")
	errValidatesWrongNames = verror.Register(
		pkgPath+".errValidatesWrongNames",
		verror.NoRetry,
		"{1:}{2:} The validated names are not a subset of the names sent by the checker{:_}")
	errValidatorIsSigner = verror.Register(
		pkgPath+".errValidatorIsSigner",
		verror.NoRetry,
		"{1:}{2:} The signature was validated by its author; treating as invalid{:_}")
	errValidatorKeyIsRevoked = verror.Register(
		pkgPath+".errValidatorKeyIsRevoked",
		verror.NoRetry,
		"{1:}{2:} The validator key is revoked{:_}")
)

// --------------------------------------------

// SignData() uses authorPrincipal to sign data using blessings (which must be
// associated with the authorPrincipal).  A pointer to a newly constructed
// DataWithSignature with IsValidated==false is returned.  Ensures that the
// blessings are stored in *cache.  Typically, "authorPrincipal" is obtained from
// v23.GetPrincipal(ctx).
//
// If a recipient of the result *d complains that it does not understand the
// hash d.BlessingHash, the signer should present it with
// blessingsData.MarshalledBlessings, which will allow the recipient to
// construct the Blessings.  The Blessings are transmitted out of line because
// they are large, and may be reused for multiple signatures.
func SignData(ctx *context.T, cache *ValidationCache, authorPrincipal security.Principal,
	blessings security.Blessings, data []Item) (d *DataWithSignature, blessingsData *BlessingsData, err error) {

	d = new(DataWithSignature)
	d.Data = data
	d.BlessingsHash, blessingsData, err = cache.AddBlessings(ctx, blessings)
	if err == nil {
		d.AuthorSigned, err = authorPrincipal.Sign(d.authorSignatureHash())
	}
	return d, blessingsData, err
}

// hashByteVectorWithLength() calls hasher.Write() on a representation of
// len(b), followed by the contents of b.
func hashByteVectorWithLength(hasher hash.Hash, b []byte) {
	var length [8]byte
	binary.LittleEndian.PutUint64(length[:], uint64(len(b)))
	hasher.Write(length[:])
	hasher.Write(b)
}

// SumByteVectorWithLength() returns a SHA-256 hash of
// len(b), followed by the contents of b.
func SumByteVectorWithLength(b []byte) []byte {
	hasher := sha256.New()
	var length [8]byte
	binary.LittleEndian.PutUint64(length[:], uint64(len(b)))
	hasher.Write(length[:])
	hasher.Write(b)
	return hasher.Sum(nil)[:]
}

// authorSignatureHash() returns the hash that the author should sign.
func (d *DataWithSignature) authorSignatureHash() []byte {
	hasher := sha256.New()
	var length [8]byte
	binary.LittleEndian.PutUint64(length[:], uint64(len(d.Data)))
	hasher.Write(length[:])
	for i := range d.Data {
		if data, gotData := d.Data[i].(ItemData); gotData {
			hasher.Write(SumByteVectorWithLength(data.Value))
		} else if hash, gotHash := d.Data[i].(ItemHash); gotHash {
			hasher.Write(hash.Value)
		} else {
			// d.Data is neither a Data nor a Hash.  This shouldn't
			// happen unless the mashalled data is somehow
			// corrupted.  The signature will not match unless the
			// original author of the data was seeing the same.
			hasher.Write([]byte("no data"))
		}
	}
	hashByteVectorWithLength(hasher, d.BlessingsHash)
	return hasher.Sum(nil)[:]
}

// validatorSignatureHash() returns the hash that the validator should sign,
// given the hash that the author signed.
func (d *DataWithSignature) validatorSignatureHash(authorSignatureHash []byte) []byte {
	var buffer [32]byte
	var buf []byte = buffer[:]
	if len(d.AuthorSigned.Hash) > len(buf) {
		buf = make([]byte, len(d.AuthorSigned.Hash))
	}
	hasher := sha256.New()
	hashByteVectorWithLength(hasher, authorSignatureHash)
	hashByteVectorWithLength(hasher, d.AuthorSigned.Purpose)
	hashByteVectorWithLength(hasher, buf[:copy(buf, d.AuthorSigned.Hash)])
	hashByteVectorWithLength(hasher, d.AuthorSigned.R)
	hashByteVectorWithLength(hasher, d.AuthorSigned.S)
	hashByteVectorWithLength(hasher, d.ValidatorDataHash)
	return hasher.Sum(nil)[:]
}

// Check() verifies the signature(s) on *d:
//
// If d.IsValidated==false, checks that:
//   1. the author's signature is available in *cache.
//   2. the author's signature over its blessings and the data is
//      cyprotgraphically valid.
//   3. security.SigningBlessingNames() yields a non-empty list of names when
//      applied to the author's blessings.
//   4. the author's public key is not known to be revoked.
//   5. the local's public key (call.LocalPrincipal().PublicKey()) is not known
//      to be revoked.
//   6. the author's public key is the public key of the RPC caller.
//   7. the author's public key and the local public key differ.
// If checks pass and there are no other errors:
//   - records the list of names found in check (3) in the ValidationData
//   - adds a validation signature using the local public key (which is now the
//     validator)
//   - sets d.IsValidated
//   - returns the list of names found in check (3), and a nil error.
// Otherwise returns a nil list of names and a non-nil error.
//
// If d.Validated==true, checks that:
//   1. the author's signature and the validator data are available in *cache.
//   2. the author's signature over its blessings and the data is
//      cyprotgraphically valid.
//   8. the list of names stored in the ValidatorData by the validator is
//      non-empty.
//   9. the author's public key and the validator's public key differ.
//  10. the list of names stored in the ValidatorData by the validator is a
//      subset of the list of names that the author's blessings could have
//      represented.
//  11. the author's public key is not known to be revoked more than
//      gracePeriod ago.
//  12. the validator's public key is not known to be revoked more than
//      gracePeriod ago.
//  13. the validator's signature is cryptographically valid.
// If checks pass and there are no other errors:
//   - returns the list of names in the validator's data, and a nil error.
// Otherwise returns a nil list of names and a non-nil error.
func (d *DataWithSignature) Check(ctx *context.T, cache *ValidationCache, call security.Call,
	krl *krl.KRL, gracePeriod time.Duration) (names []string, err error) {

	// Verify that we have the Blessings and ValidatorData.
	var authorBlessingsData *BlessingsData = cache.LookupBlessingsData(ctx, d.BlessingsHash)
	var validatorData *ValidatorData
	if d.IsValidated {
		validatorData = cache.LookupValidatorData(ctx, d.ValidatorDataHash)
	}
	if authorBlessingsData == nil || (validatorData == nil && d.IsValidated) { // Check (1).
		if authorBlessingsData == nil && (validatorData == nil && d.IsValidated) {
			err = verror.New(ErrNeedAuthorBlessingsAndValidatorDataForHash, ctx)
		} else if authorBlessingsData == nil {
			err = verror.New(ErrNeedAuthorBlessingsForHash, ctx)
		} else {
			err = verror.New(ErrNeedValidatorDataForHash, ctx)
		}
	}

	// Check the author signature.
	var authorSignatureHash []byte
	if err == nil {
		authorSignatureHash = d.authorSignatureHash()
		if !d.AuthorSigned.Verify(authorBlessingsData.UnmarshalledBlessings.PublicKey(), authorSignatureHash) { // Check (2).
			err = verror.New(errBadAuthorSignature, ctx)
		}
	}

	// Check or create the validator signature.
	now := time.Now()
	if err != nil {
		// err already set
	} else if !d.IsValidated {
		// Not yet validated, so this run will attempt to validate.
		var validatedNames []string
		var localKeyMarshalled []byte
		var senderKeyMarshalled []byte
		validatedNames, _ = security.SigningBlessingNames(ctx, call.LocalPrincipal(),
			authorBlessingsData.UnmarshalledBlessings)
		if len(validatedNames) == 0 { // Check (3).
			err = verror.New(errAuthorBlessingsHaveNoValidNames, ctx)
		} else if localKeyMarshalled, err = call.LocalPrincipal().PublicKey().MarshalBinary(); err != nil {
			// err already set
		} else if krl.RevocationTime(authorBlessingsData.MarshalledPublicKey).Before(now) { // Check (4).
			err = verror.New(errAuthorKeyIsRevoked, ctx)
		} else if krl.RevocationTime(localKeyMarshalled).Before(now) { // Check (5).
			err = verror.New(errValidatorKeyIsRevoked, ctx)
		} else if senderKeyMarshalled, err = call.RemoteBlessings().PublicKey().MarshalBinary(); err != nil {
			// err already set
		} else if !bytes.Equal(senderKeyMarshalled, authorBlessingsData.MarshalledPublicKey) { // Check (6).
			err = verror.New(errSenderIsNotAuthor, ctx)
		} else if bytes.Equal(localKeyMarshalled, authorBlessingsData.MarshalledPublicKey) { // Check (7).
			err = verror.New(errMayNotValidateOwnSignature, ctx)
		} else {
			// Local principal is different from author, so can validate.
			validatorData = &ValidatorData{
				Names:               validatedNames,
				PublicKey:           call.LocalPrincipal().PublicKey(),
				MarshalledPublicKey: localKeyMarshalled,
			}
			d.ValidatorDataHash = cache.AddValidatorData(ctx, validatorData)
			d.ValidatorSigned, err = call.LocalPrincipal().Sign(d.validatorSignatureHash(authorSignatureHash))
			d.IsValidated = (err == nil)
		}
	} else { // Data already validated; check the validator siganture.
		if len(validatorData.Names) == 0 { // Check (8).
			err = verror.New(errAuthorBlessingsHaveNoValidNames, ctx)
		} else if bytes.Equal(validatorData.MarshalledPublicKey, authorBlessingsData.MarshalledPublicKey) { // Check (9).
			err = verror.New(errValidatorIsSigner, ctx)
		} else if !authorBlessingsData.UnmarshalledBlessings.CouldHaveNames(validatorData.Names) { // Check (10).
			err = verror.New(errValidatesWrongNames, ctx)
		} else if krl.RevocationTime(authorBlessingsData.MarshalledPublicKey).Before(now.Add(-gracePeriod)) { // Check (11).
			err = verror.New(errAuthorKeyIsRevoked, ctx)
		} else if krl.RevocationTime(validatorData.MarshalledPublicKey).Before(now.Add(-gracePeriod)) { // Check (12).
			err = verror.New(errValidatorKeyIsRevoked, ctx)
		} else if !d.ValidatorSigned.Verify(validatorData.PublicKey, d.validatorSignatureHash(authorSignatureHash)) { // Check (13).
			err = verror.New(errBadValidatorSignature, ctx)
		} // else success.
	}

	// If there were no errors, return the list of names from the validator.
	if err == nil {
		names = make([]string, len(validatorData.Names))
		copy(names, validatorData.Names)
	}

	return names, err
}
