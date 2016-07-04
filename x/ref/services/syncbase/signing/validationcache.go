// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This module implements a cache of data associated with the
// signatures, keyed by hash values of the data.  The intent is that
// communicating devices will refer to the data using hashes, and transmit the
// data itself only if the device on the other side does not have the data in
// its cache.

package signing

import "crypto/sha256"
import "encoding/binary"
import "time"

import "v.io/x/ref/services/syncbase/signing/hashcache"
import "v.io/v23/context"
import "v.io/v23/security"
import "v.io/v23/vom"

// --------------------------------------------

// A BlessingsData contains information about a security.Blessings object.  The
// object itself is referred to by UnmarshalledBlessings.  The implementation
// constructs all instances; the client should not modify fields.
type BlessingsData struct {
	UnmarshalledBlessings security.Blessings // The Blessings.
	MarshalledBlessings   []byte             // VOM encoded Blessings.
	MarshalledPublicKey   []byte             // Value from blessings.PublicKey().MarshalBinary().
}

// A ValidatorData is the extra data that a validator signs when validating and
// signing a DataWithSignature.  Clients may construct instances to pass to
// AddValidatorData(), but should not modify the fields of a constructed
// ValidatorData.
type ValidatorData struct {
	Names               []string           // Names of valid signing blessings in the Blessings referred to by BlessingsHash.
	PublicKey           security.PublicKey // The key used to create ValidatorSigned.
	MarshalledPublicKey []byte             // PublicKey, marshalled with MarshalBinary().
}

// hash() returns the hash of *vd.  This hash should be used in the
// ValidatorDataHash field of DataWithSignature, and as the cache key of *vd
// in a ValidationCache.
func (vd *ValidatorData) hash() []byte {
	hasher := sha256.New()
	var buffer [256]byte
	var buf []byte = buffer[:]
	binary.LittleEndian.PutUint64(buf[:], uint64(len(vd.Names)))
	hasher.Write(buf[:8])
	for i := range vd.Names {
		if len(vd.Names[i]) > len(buf) {
			buf = make([]byte, len(vd.Names[i])+256)
		}
		hashByteVectorWithLength(hasher, []byte(vd.Names[i]))
	}
	hashByteVectorWithLength(hasher, vd.MarshalledPublicKey)
	return hasher.Sum(nil)[:]
}

// A ValidationCache records recently-seen instances of BlessingsData and
// ValidatorData values, keys by hashes of the blessings and validator keys
// respectively.  Values may expire from the cache if unused for a duration
// specified with NewValidationCache().
type ValidationCache struct {
	blessingsCache *hashcache.Cache
	validatorCache *hashcache.Cache
}

// NewValidationCache() returns a pointer to a new, empty ValidationCache with
// the specified expiry duration..
func NewValidationCache(expiry time.Duration) *ValidationCache {
	return &ValidationCache{
		blessingsCache: hashcache.New(expiry),
		validatorCache: hashcache.New(expiry)}
}

// LookupBlessingsData() returns a pointer to the BlessingsData associated with
// blessingsHash in *vc.  blessingsHash should have been returned by a previous
// call to AddBlessings() or AddWireBlessings() (possibly on another machine).
// nil is returned if the data is not present.  The client should not modify
// *result, since it is shared with *vc.
func (vc *ValidationCache) LookupBlessingsData(ctx *context.T, blessingsHash []byte) (result *BlessingsData) {
	value, found := vc.blessingsCache.Lookup(blessingsHash)
	if found {
		result = value.(*BlessingsData)
	}
	return result
}

// addBlessings() adds a BlessingsData for blessings to *vc, and returns a hash
// value, which if passed to LookupBlessingsData() will yield a pointer to the
// BlessingsData, or a non-nil error.  The fields of BlessingsData other than
// MarshalledBlessings and UnmarshalledBlessings are constructed by this
// routine.  Requires that blessings and marshalledBlessings represent the same
// data, or that marshalledBlessings be nil.
func (vc *ValidationCache) addBlessings(ctx *context.T, blessings security.Blessings,
	marshalledBlessings []byte) (blessingsHash []byte, data *BlessingsData, err error) {

	blessingsHash = blessings.UniqueID()
	if value, found := vc.blessingsCache.Lookup(blessingsHash); found {
		data = value.(*BlessingsData)
	} else { // not found
		var marshalledKey []byte
		if marshalledBlessings == nil {
			marshalledBlessings, err = vom.Encode(blessings)
		}
		if err == nil {
			marshalledKey, err = blessings.PublicKey().MarshalBinary()
		}
		if err == nil {
			data = &BlessingsData{
				UnmarshalledBlessings: blessings,
				MarshalledBlessings:   marshalledBlessings,
				MarshalledPublicKey:   marshalledKey}
			vc.blessingsCache.Add(blessingsHash, data)
		}
	}
	return blessingsHash, data, err
}

// AddBlessings() adds a BlessingsData for blessings to *cv, and
// returns a hash value, which if passed to LookupBlessingsData() will yield a
// pointer to the BlessingsData, or a non-nil error.  The fields of
// BlessingsData other than UnmarshalledBlessings are constructed by this
// routine.
func (vc *ValidationCache) AddBlessings(ctx *context.T, blessings security.Blessings) (blessingsHash []byte, data *BlessingsData, err error) {
	return vc.addBlessings(ctx, blessings, nil)
}

// AddWireBlessings() adds a BlessingsData for blessings to *cv, and
// returns a hash value, which if passed to LookupBlessingsData() will yield a
// pointer to the BlessingsData, or a non-nil error.  The fields of
// BlessingsData other than MarshalledBlessings are constructed by this
// routine.
func (vc *ValidationCache) AddWireBlessings(ctx *context.T,
	marshalledBlessings []byte) (blessingsHash []byte, data *BlessingsData, err error) {

	var blessings security.Blessings
	err = vom.Decode(marshalledBlessings, &blessings)
	if err == nil {
		blessingsHash, data, err = vc.addBlessings(ctx, blessings, marshalledBlessings)
	}
	return blessingsHash, data, err
}

// LookupValidatorData() returns a pointer to the ValidatorData associated with
// hash validatorHash in *vc validatorHash should have been returned by a
// previous call to AddValidatorData() (possibly on another machine).  nil is
// returned if the data is not present.  The client should not modifiy *result,
// since it it shared with *vc.
func (vc *ValidationCache) LookupValidatorData(ctx *context.T, validatorHash []byte) (result *ValidatorData) {
	value, found := vc.validatorCache.Lookup(validatorHash)
	if found {
		result = value.(*ValidatorData)
	}
	return result
}

// AddValidatorData() adds a ValidatorData *vd to cache *vc, and returns a hash
// value, which if passed to LookupValidatorData() will yield a pointer to the
// ValidatorData.  The client should not modify *vd after the call, since it is
// shared with *vc.
func (vc *ValidationCache) AddValidatorData(ctx *context.T, vd *ValidatorData) (validatorDataHash []byte) {
	validatorDataHash = vd.hash()
	vc.validatorCache.Add(validatorDataHash, vd)
	return validatorDataHash
}

// ToWireValidatorData() puts the wire form of ValidatorData *vd in *wvd.
func ToWireValidatorData(vd *ValidatorData) (wvd WireValidatorData) {
	wvd.Names = make([]string, len(vd.Names))
	copy(wvd.Names, vd.Names)
	wvd.MarshalledPublicKey = make([]byte, len(vd.MarshalledPublicKey))
	copy(wvd.MarshalledPublicKey, vd.MarshalledPublicKey)
	return wvd
}

// FromWireValidatorData() puts the in-memory form of WireValidatorData *wvd in *vd.
func FromWireValidatorData(wvd *WireValidatorData) (vd ValidatorData, err error) {
	vd.PublicKey, err = security.UnmarshalPublicKey(wvd.MarshalledPublicKey)
	if err == nil {
		vd.Names = make([]string, len(wvd.Names))
		copy(vd.Names, wvd.Names)
		vd.MarshalledPublicKey = make([]byte, len(wvd.MarshalledPublicKey))
		copy(vd.MarshalledPublicKey, wvd.MarshalledPublicKey)
	}
	return vd, err
}
