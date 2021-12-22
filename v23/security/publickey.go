// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding"
	"fmt"
)

// PublicKey represents a public key using an unspecified algorithm.
//
// MarshalBinary returns the DER-encoded PKIX representation of the public key,
// while UnmarshalPublicKey creates a PublicKey object from the marshaled bytes.
//
// String returns a human-readable representation of the public key.
//
// The implementations of PublicKey are responsible for specifying a hashing
// algorithm used for computing chained digests that are then signed. Although
// each Signature type specifies the hash function used to compute it there
// is no explicit means of doing so for when the signatures themselves form a
// chain. The convention adopted is that the implementations of this interface
// makes that choice and hence the public key required to verify a signature
// chain is sufficient to determine the hash function to use. This assumes
// that the implementation choices do not change. In summary they are as
// follows:
// ecsda: the hash function is based on the curve:
//        <= 160: SHA1, <= 256: SHA256, <= 384: SHA384, all others SHA512.
// ed25519 and rsa always use SHA512.
//
// The implementations of PublicKey can verify a Signature using the hash
// function specified in the Signature.
type PublicKey interface {
	encoding.BinaryMarshaler
	fmt.Stringer

	// hash returns a cryptographic hash function whose security strength is
	// appropriate for creating message digests to sign with this public key.
	// For example, an ECDSA public key with a 512-bit curve would require a
	// 512-bit hash function, whilst a key with a 256-bit curve would be
	// happy with a 256-bit hash function.
	hash() crypto.Hash

	messageDigest(h crypto.Hash, purpose, message []byte) []byte

	// verify returns true iff signature was created by the corresponding
	// private key when signing the provided message digest (obtained by
	// the messageDigest function).
	verify(digest []byte, signature *Signature) bool
}

// UnmarshalPublicKey returns a PublicKey object from the DER-encoded PKIX
// represntation of it (typically obtianed via PublicKey.MarshalBinary).
func UnmarshalPublicKey(bytes []byte) (PublicKey, error) {
	key, err := x509.ParsePKIXPublicKey(bytes)
	if err != nil {
		return nil, err
	}
	switch v := key.(type) {
	case *ecdsa.PublicKey:
		return newECDSAPublicKeyImpl(v, ecdsaHash(v)), nil
	case ed25519.PublicKey:
		return newED25519PublicKeyImpl(v, SHA512Hash), nil
	case *rsa.PublicKey:
		return newRSAPublicKeyImpl(v, SHA512Hash), nil
	default:
		return nil, fmt.Errorf("unrecognized PublicKey type %T", key)
	}
}

type publicKeyCommon struct {
	vhash       Hash
	chash       crypto.Hash
	keyBytes    []byte
	keyBytesErr error
}

func newPublicKeyCommon(key interface{}, hash Hash) publicKeyCommon {
	kb, kbe := x509.MarshalPKIXPublicKey(key)
	return publicKeyCommon{
		vhash:       hash,
		chash:       cryptoHash(hash),
		keyBytes:    kb,
		keyBytesErr: kbe,
	}
}

func (pk publicKeyCommon) hash() crypto.Hash {
	return pk.chash
}

func (pk publicKeyCommon) MarshalBinary() ([]byte, error) {
	if err := pk.keyBytesErr; err != nil {
		return nil, err
	}
	n := make([]byte, len(pk.keyBytes))
	copy(n, pk.keyBytes)
	return n, nil
}

func (pk publicKeyCommon) String() string {
	const hextable = "0123456789abcdef"
	hash := md5.Sum(pk.keyBytes)
	var repr [md5.Size * 3]byte
	for i, v := range hash {
		repr[i*3] = hextable[v>>4]
		repr[i*3+1] = hextable[v&0x0f]
		repr[i*3+2] = ':'
	}
	return string(repr[:len(repr)-1])
}

// NewPublicKey creates a new security.PublicKey for the supplied
// crypto.PublicKey.
func NewPublicKey(key crypto.PublicKey) (PublicKey, error) {
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		return NewECDSAPublicKey(k), nil
	case *rsa.PublicKey:
		return NewRSAPublicKey(k), nil
	case ed25519.PublicKey:
		return NewED25519PublicKey(k), nil
	}
	return nil, fmt.Errorf("%T is an unsupported key type", key)
}
