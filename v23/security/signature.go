// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
)

// Verify returns true iff sig is a valid signature for a message.
func (sig *Signature) Verify(key PublicKey, message []byte) bool {
	if message = key.messageDigest(sig.Purpose, message); message == nil {
		return false
	}
	return key.verify(message, sig)
}

// Digest returns a hash that is computed over all of the fields of the
// signature that are used for a particular signature algorithm. The
// digest can be used as a cache key or for 'chaining' as per
// Certificate.chainedDigests.
func (sig *Signature) digest(hashfn Hash) []byte {
	var fields []byte
	w := func(data []byte) {
		fields = append(fields, hashfn.sum(data)...)
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
	return hashfn.sum(fields)
}

// messageDigestFields returns a concatenation of the hashes of each field
// which consits of the key, the purpose and the message.
// In order to defend against "Duplicate Signature Key Selection (DSKS)" attacks
// as defined in the paper "Another look at Security Definition" by Neal Koblitz
// and Alfred Menzes, we also include the public key of the signer in the message
// being signed.
func messageDigestFields(hash Hash, publicKeyBytes, purpose, message []byte) []byte {
	numFields := 3
	fields := make([]byte, 0, hash.size()*numFields)
	w := func(data []byte) bool {
		h := hash.sum(data)
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

func messageDigest(hash Hash, purpose, message []byte, key PublicKey) []byte {
	keyBytes, err := key.MarshalBinary()
	if err != nil {
		return nil
	}
	return hash.sum(messageDigestFields(hash, keyBytes, purpose, message))
}

// sum returns the hash of data using hash as the cryptographic hash function.
// Returns nil if the hash function is not recognized.
func (hash Hash) sum(data []byte) []byte {
	switch hash {
	case SHA1Hash:
		h := sha1.Sum(data)
		return h[:]
	case SHA256Hash:
		h := sha256.Sum256(data)
		return h[:]
	case SHA384Hash:
		h := sha512.Sum384(data)
		return h[:]
	case SHA512Hash:
		h := sha512.Sum512(data)
		return h[:]
	}
	return nil
}

func (hash Hash) size() int {
	switch hash {
	case SHA1Hash:
		return sha1.Size
	case SHA256Hash:
		return sha256.Size
	case SHA384Hash:
		return sha512.Size384
	case SHA512Hash:
		return sha512.Size
	}
	return 0
}
