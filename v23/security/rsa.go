package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
)

// NewRSAPublicKey creates a PublicKey object that uses the RSA algorithm and the provided RSA public key.
func NewRSAPublicKey(key *rsa.PublicKey) PublicKey {
	if key.Size() < (2048 / 8) {
		panic("rsa keys with less than 2048 bits are not supported")
	}
	return newRSAPublicKeyImpl(key)
}

type rsaPublicKey struct {
	key *rsa.PublicKey
}

func (pk *rsaPublicKey) MarshalBinary() ([]byte, error) { return x509.MarshalPKIXPublicKey(pk.key) }
func (pk *rsaPublicKey) String() string                 { return publicKeyString(pk) }
func (pk *rsaPublicKey) verify(digest []byte, sig *Signature) bool {
	err := rsa.VerifyPKCS1v15(pk.key, cryptoHash(pk.key.Size()), digest, sig.Rsa)
	return err == nil
}

func cryptoHash(nbytes int) crypto.Hash {
	if nbytes < (2048 / 8) {
		panic("rsa keys with less than 2048 bits are not supported")
	}
	if nbytes == (2048 / 8) {
		return crypto.SHA256
	}
	return crypto.SHA512
}

func (pk *rsaPublicKey) hash() Hash {
	if pk.key.Size() == (2048 / 8) {
		return SHA256Hash
	}
	return SHA512Hash
}

// NewInMemoryRSASigner creates a Signer that uses the provided RSA
// private  key to sign messages.  This private key is kept in the clear in
// the memory of the running process.
func NewInMemoryRSASigner(key *rsa.PrivateKey) (Signer, error) {
	signer, err := newInMemoryRSASignerImpl(key)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

// NewRSASigner creates a Signer that uses the provided function to sign
// messages.
func NewRSASigner(key *rsa.PublicKey, sign func(data []byte) ([]byte, error)) Signer {
	return &rsaSigner{sign: sign, pubkey: NewRSAPublicKey(key)}
}

type rsaSigner struct {
	sign   func(data []byte) (sig []byte, err error)
	pubkey PublicKey
	// Object to hold on to for garbage collection
	impl interface{} //nolint:structcheck,unused
}

func (c *rsaSigner) Sign(purpose, message []byte) (Signature, error) {
	hash := c.pubkey.hash()
	if message = messageDigest(hash, purpose, message, c.pubkey); message == nil {
		return Signature{}, fmt.Errorf("unable to create bytes to sign from message with hashing function: %v", hash)
	}
	sig, err := c.sign(message)
	if err != nil {
		return Signature{}, err
	}
	return Signature{
		Purpose: purpose,
		Hash:    hash,
		Rsa:     sig,
	}, nil
}

func (c *rsaSigner) PublicKey() PublicKey {
	return c.pubkey
}

func newGoStdlibRSASigner(key *rsa.PrivateKey) (Signer, error) {
	hash := cryptoHash(key.PublicKey.Size())
	sign := func(data []byte) ([]byte, error) {
		return rsa.SignPKCS1v15(rand.Reader, key, hash, data)
	}
	return &rsaSigner{sign: sign, pubkey: newGoStdlibRSAPublicKey(&key.PublicKey)}, nil
}

func newGoStdlibRSAPublicKey(key *rsa.PublicKey) PublicKey {
	return &rsaPublicKey{key}
}
