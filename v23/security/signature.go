// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
)

// Verify returns true iff sig is a valid signature for a message.
func (sig *Signature) Verify(key PublicKey, message []byte) bool {
	if !sig.X509 {
		if message = key.messageDigest(cryptoHash(sig.Hash), sig.Purpose, message); message == nil {
			return false
		}
	}
	return key.verify(message, sig)
}

// messageDigestFields returns a concatenation of the hashes of each field
// which consits of the key, the purpose and the message.
// In order to defend against "Duplicate Signature Key Selection (DSKS)" attacks
// as defined in the paper "Another look at Security Definition" by Neal Koblitz
// and Alfred Menzes, we also include the public key of the signer in the message
// being signed.
func messageDigestFields(hash crypto.Hash, publicKeyBytes, purpose, message []byte) []byte {
	numFields := 3
	fields := make([]byte, 0, hash.Size()*numFields)
	w := func(data []byte) bool {
		h := sum(hash, data)
		if h == nil {
			return false
		}
		fields = append(fields, h...)
		return true
	}
	if !w(publicKeyBytes) {
		return nil
	}
	if !w(message) {
		return nil
	}
	if !w(purpose) {
		return nil
	}
	return fields
}

func messageDigest(hash crypto.Hash, purpose, message []byte, key PublicKey) []byte {
	keyBytes, err := key.MarshalBinary()
	if err != nil {
		return nil
	}
	return sum(hash, messageDigestFields(hash, keyBytes, purpose, message))
}

// Digest returns a hash that is computed over all of the fields of the
// signature that are used for a particular signature algorithm. The
// digest can be used as a cache key or for 'chaining' as per
// Certificate.chainedDigests.
func (sig *Signature) digest(hashfn crypto.Hash) []byte {
	var fields []byte
	w := func(data []byte) {
		fields = append(fields, sum(hashfn, data)...)
	}
	w([]byte(sig.Hash))
	w(sig.Purpose)
	switch {
	case len(sig.R) > 0:
		w([]byte("ECDSA")) // The signing algorithm
		w(sig.R)
		w(sig.S)
	case len(sig.Ed25519) > 0:
		w([]byte("ED25519")) // The signing algorithm
		w(sig.Ed25519)
	default:
		w([]byte("RSA")) // The signing algorithm
		w(sig.Rsa)
	}
	if sig.X509 {
		w([]byte{0x01})
	}
	return sum(hashfn, fields)
}

func sum(hash crypto.Hash, data []byte) []byte {
	switch hash {
	case crypto.SHA1:
		h := sha1.Sum(data)
		return h[:]
	case crypto.SHA256:
		h := sha256.Sum256(data)
		return h[:]
	case crypto.SHA384:
		h := sha512.Sum384(data)
		return h[:]
	case crypto.SHA512:
		h := sha512.Sum512(data)
		return h[:]
	}
	return nil
}

func cryptoHash(h Hash) crypto.Hash {
	switch h {
	case SHA1Hash:
		return crypto.SHA1
	case SHA256Hash:
		return crypto.SHA256
	case SHA384Hash:
		return crypto.SHA384
	case SHA512Hash:
		return crypto.SHA512
	}
	return crypto.SHA512
}

type signerCommon struct {
	pubkey PublicKey
	chash  crypto.Hash
	vhash  Hash
}

func newSignerCommon(pk PublicKey, hash Hash) signerCommon {
	return signerCommon{
		pubkey: pk,
		vhash:  hash,
		chash:  cryptoHash(hash),
	}
}

func (sc *signerCommon) hash() crypto.Hash {
	return sc.chash
}
