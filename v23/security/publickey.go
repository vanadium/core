// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
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
type PublicKey interface {
	encoding.BinaryMarshaler
	fmt.Stringer

	// hash returns a cryptographic hash function whose security strength is
	// appropriate for creating message digests to sign with this public key.
	// For example, an ECDSA public key with a 512-bit curve would require a
	// 512-bit hash function, whilst a key with a 256-bit curve would be
	// happy with a 256-bit hash function.
	hash() Hash

	messageDigest(purpose, message []byte) []byte

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
		return newECDSAPublicKeyImpl(v), nil
	case ed25519.PublicKey:
		return newED25519PublicKeyImpl(v), nil
	case *rsa.PublicKey:
		return newRSAPublicKeyImpl(v), nil
	default:
		return nil, fmt.Errorf("unrecognized PublicKey type %T", key)
	}
}

type publicKeyCommon struct {
	keyBytes    []byte
	keyBytesErr error
	h           Hash
}

func newPublicKeyCommon(h Hash, key interface{}) publicKeyCommon {
	kb, kbe := x509.MarshalPKIXPublicKey(key)
	return publicKeyCommon{
		keyBytes:    kb,
		keyBytesErr: kbe,
		h:           h,
	}
}

func (pk publicKeyCommon) hash() Hash {
	return pk.h
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
