// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"math/big"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/internal"
)

// IsSupported returns true if the suplied ssh key type is supported.
func IsSupported(key ssh.PublicKey) bool {
	switch key.Type() {
	case ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521:
		return true
	case ssh.KeyAlgoED25519:
		return true
	}
	return false
}

// hashForSSHKey returns the hash function to be used for the supplied
// ssh key. It assumes the specified key is of a supported type.
func hashForSSHKey(key ssh.PublicKey) (security.Hash, hash.Hash) {
	switch key.Type() {
	case ssh.KeyAlgoECDSA256:
		return security.SHA256Hash, sha256.New()
	case ssh.KeyAlgoECDSA384:
		return security.SHA384Hash, sha512.New384()
	case ssh.KeyAlgoECDSA521:
		return security.SHA512Hash, sha512.New()
	case ssh.KeyAlgoED25519:
		return security.SHA512Hash, sha512.New()
	}
	return "", nil
}

// FromECDSAKey creates a security.PublicKey from an ssh ECDSA key.
func FromECDSAKey(key ssh.PublicKey) (security.PublicKey, error) {
	k, err := internal.ParseECDSAKey(key)
	if err != nil {
		return nil, err
	}
	return security.NewECDSAPublicKey(k), nil
}

// FromECDSAKey creates a security.PublicKey from an ssh ED25519 key.
func FromED25512Key(key ssh.PublicKey) (security.PublicKey, error) {
	k, err := internal.ParseED25519Key(key)
	if err != nil {
		return nil, err
	}
	return security.NewED25519PublicKey(k), nil
}

func digest(hasher hash.Hash, messages ...[]byte) ([]byte, error) {
	hashes := []byte{}

	hashField := func(data []byte) error {
		if _, err := hasher.Write(data); err != nil {
			return fmt.Errorf("failed to hash field: %v", err)
		}
		h := hasher.Sum(nil)
		hashes = append(hashes, h...)
		hasher.Reset()
		return nil
	}

	for _, msg := range messages {
		if err := hashField(msg); err != nil {
			return nil, err
		}
	}
	return hashes, nil
}

// DigestsForSSH returns a concatenation of the hashes of the public key,
// the message and the purpose. The openSSH and openSSL ECDSA code will hash this
// message again internally and hence these hashes must not be themselves
// hashed here to ensure compatibility with the Vanadium signature verification
// which uses the go crypto code so that a messages signed by the SSH agent/ssl
// code can be verified by the Vanadium code.
func DigestsForSSH(sshPK ssh.PublicKey, v23PK, purpose, message []byte) ([]byte, security.Hash, error) {
	hashName, hasher := hashForSSHKey(sshPK)
	sum, err := digest(hasher, v23PK, message, purpose)
	if err != nil {
		return nil, hashName, err
	}
	return sum, hashName, nil
}

// HashedDigestsForSSH hashes the digests returned by DigestsForSSH using
// an appropriate hash function for the ssh key. The ED25519 implementation
// in openSSH does not rehash internally and consequently this is needed
// for compatibility with the Vanadium code.
func HashedDigestsForSSH(sshPK ssh.PublicKey, v23PK, purpose, message []byte) ([]byte, security.Hash, error) {
	hashName, hasher := hashForSSHKey(sshPK)
	sum, err := digest(hasher, v23PK, message, purpose)
	if err != nil {
		return nil, hashName, err
	}
	hasher.Reset()
	hasher.Write(sum)
	return hasher.Sum(nil), hashName, nil
}

// UnmarshalSSHECDSASignature unmarshals the R and S signature components
// from the returned signature.
func UnmarshalSSHECDSASignature(sig *ssh.Signature) (r, s []byte, err error) {
	var ecSig struct {
		R, S *big.Int
	}
	if err := ssh.Unmarshal(sig.Blob, &ecSig); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal ECDSA signature: %v", err)
	}
	return ecSig.R.Bytes(), ecSig.S.Bytes(), nil
}
