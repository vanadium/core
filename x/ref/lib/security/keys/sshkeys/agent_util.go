// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"math/big"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
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
	k, err := parseECDSAKey(key)
	if err != nil {
		return nil, err
	}
	return security.NewECDSAPublicKey(k), nil
}

// fromED25512Key creates a security.PublicKey from an ssh ED25519 key.
func fromED25512Key(key ssh.PublicKey) (security.PublicKey, error) {
	k, err := parseED25519Key(key)
	if err != nil {
		return nil, err
	}
	return security.NewED25519PublicKey(k), nil
}

// fromRSAAKey creates a security.PublicKey from an ssh RSA key.
func fromRSAKey(key ssh.PublicKey) (security.PublicKey, error) {
	k, err := parseRSAKey(key)
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

// parseECDSAKey creates an ecdsa.PublicKey from an ssh ECDSA key.
func parseECDSAKey(key ssh.PublicKey) (*ecdsa.PublicKey, error) {
	var sshWire struct {
		Name string
		ID   string
		Key  []byte
	}
	if err := ssh.Unmarshal(key.Marshal(), &sshWire); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key type: %v: %v", key.Type(), err)
	}
	pk := new(ecdsa.PublicKey)
	switch sshWire.ID {
	case "nistp256":
		pk.Curve = elliptic.P256()
	case "nistp384":
		pk.Curve = elliptic.P384()
	case "nistp521":
		pk.Curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("uncrecognised ecdsa curve: %v", sshWire.ID)
	}
	pk.X, pk.Y = elliptic.Unmarshal(pk.Curve, sshWire.Key)
	if pk.X == nil || pk.Y == nil {
		return nil, fmt.Errorf("invalid curve point")
	}
	return pk, nil
}

// parseED25519Key creates an ed25519.PublicKey from an ssh ED25519 key.
func parseED25519Key(key ssh.PublicKey) (ed25519.PublicKey, error) {
	var sshWire struct {
		Name     string
		KeyBytes []byte
	}
	if err := ssh.Unmarshal(key.Marshal(), &sshWire); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key %v: %v", key.Type(), err)
	}
	return ed25519.PublicKey(sshWire.KeyBytes), nil
}

// parseRSAKey creates an rsa.PublicKey from an ssh rsa key.
func parseRSAKey(key ssh.PublicKey) (*rsa.PublicKey, error) {
	var sshWire struct {
		Name string
		E    *big.Int
		N    *big.Int
	}
	if err := ssh.Unmarshal(key.Marshal(), &sshWire); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key %v: %v", key.Type(), err)
	}
	return &rsa.PublicKey{N: sshWire.N, E: int(sshWire.E.Int64())}, nil
}
