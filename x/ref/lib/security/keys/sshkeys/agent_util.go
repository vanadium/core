// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/internal"
)

type hashes struct {
	v23hash security.Hash
	gohash  func() hash.Hash
}

var supportedKeyTypesMap = map[string]hashes{
	ssh.KeyAlgoECDSA256: {security.SHA256Hash, sha256.New},
	ssh.KeyAlgoECDSA384: {security.SHA384Hash, sha512.New384},
	ssh.KeyAlgoECDSA521: {security.SHA512Hash, sha512.New},
	ssh.KeyAlgoED25519:  {security.SHA512Hash, sha512.New},
	ssh.KeyAlgoRSA:      {security.SHA512Hash, sha512.New},
}

// isSupported returns true if the suplied ssh key type is supported.
func isSupported(key ssh.PublicKey) bool {
	_, ok := supportedKeyTypesMap[key.Type()]
	return ok
}

// supportedKeyTypes retruns
func supportedKeyTypes() []string {
	r := []string{}
	for k := range supportedKeyTypesMap {
		r = append(r, k)
	}
	return r
}

// hashForSSHKey returns the hash function to be used for the supplied
// ssh key. It assumes the specified key is of a supported type.
func hashForSSHKey(key ssh.PublicKey) (security.Hash, hash.Hash) {
	if h, ok := supportedKeyTypesMap[key.Type()]; ok {
		return h.v23hash, h.gohash()
	}
	return "", nil
}

// fromECDSAKey creates a security.PublicKey from an ssh ECDSA key.
func fromECDSAKey(key ssh.PublicKey) (security.PublicKey, error) {
	k, err := internal.ParseECDSAKey(key)
	if err != nil {
		return nil, err
	}
	return security.NewECDSAPublicKey(k), nil
}

// fromED25512Key creates a security.PublicKey from an ssh ED25519 key.
func fromED25512Key(key ssh.PublicKey) (security.PublicKey, error) {
	k, err := internal.ParseED25519Key(key)
	if err != nil {
		return nil, err
	}
	return security.NewED25519PublicKey(k), nil
}

// fromRSAAKey creates a security.PublicKey from an ssh RSA key.
func fromRSAKey(key ssh.PublicKey) (security.PublicKey, error) {
	k, err := internal.ParseRSAKey(key)
	if err != nil {
		return nil, err
	}
	return security.NewRSAPublicKey(k), nil
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

// digestsForSSH returns a concatenation of the hashes of the public key,
// the message and the purpose. The openSSH and openSSL ECDSA code will hash this
// message again internally and hence these hashes must not be themselves
// hashed here to ensure compatibility with the Vanadium signature verification
// which uses the go crypto code so that a messages signed by the SSH agent/ssl
// code can be verified by the Vanadium code.
func digestsForSSH(sshPK ssh.PublicKey, v23PK, purpose, message []byte) ([]byte, security.Hash, error) {
	hashName, hasher := hashForSSHKey(sshPK)
	sum, err := digest(hasher, v23PK, message, purpose)
	if err != nil {
		return nil, hashName, err
	}
	return sum, hashName, nil
}

// hashedDigestsForSSH hashes the digests returned by DigestsForSSH using
// an appropriate hash function for the ssh key. The ED25519 implementation
// in openSSH does not rehash internally and consequently this is needed
// for compatibility with the Vanadium code.
func hashedDigestsForSSH(sshPK ssh.PublicKey, v23PK, purpose, message []byte) ([]byte, security.Hash, error) {
	hashName, hasher := hashForSSHKey(sshPK)
	sum, err := digest(hasher, v23PK, message, purpose)
	if err != nil {
		return nil, hashName, err
	}
	hasher.Reset()
	hasher.Write(sum)
	return hasher.Sum(nil), hashName, nil
}
