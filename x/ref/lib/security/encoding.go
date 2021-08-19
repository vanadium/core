// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/ref/lib/security/internal"
)

// DecodeBlessingsBase64 decodes blessings from the supplied base64
// url encoded string.
func DecodeBlessingsBase64(encoded string) (security.Blessings, error) {
	var b security.Blessings
	if err := base64urlVomDecode(encoded, &b); err != nil {
		return security.Blessings{}, fmt.Errorf("failed to decode %v: %v", encoded, err)
	}
	return b, nil
}

func base64urlVomDecode(s string, i interface{}) error {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return vom.Decode(b, i)
}

// EncodeBlessingsBase64 encodes the supplied blessings as a base 64
// url encoded string.
func EncodeBlessingsBase64(blessings security.Blessings) (string, error) {
	if blessings.IsZero() {
		return "", fmt.Errorf("no blessings found")
	}
	str, err := base64urlVomEncode(blessings)
	if err != nil {
		return "", fmt.Errorf("base64url-vom encoding failed: %v", err)
	}
	return str, nil
}

func base64urlVomEncode(i interface{}) (string, error) {
	buf, err := vom.Encode(i)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// EncodePublicKeyBase64 encodes the supplied public key as a base64
// url encoded string. The underlying data format is DER.
func EncodePublicKeyBase64(key security.PublicKey) (string, error) {
	buf, err := key.MarshalBinary()
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// DecodePublicKeyBase64 decodes a public key from the supplied
// base64 url encoded string. It assumes that the underlying data format
// is DER.
func DecodePublicKeyBase64(key string) (security.PublicKey, error) {
	buf, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	return security.UnmarshalPublicKey(buf)
}

func publicKeyFromCryptoKey(key interface{}) (security.PublicKey, error) {
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		return security.NewECDSAPublicKey(k), nil
	case ed25519.PublicKey:
		return security.NewED25519PublicKey(k), nil
	}
	return nil, fmt.Errorf("unsupported key type %T", key)
}

// DecodePublicKeyPEM decodes the supplied PEM block to obtain a public key.
func DecodePublicKeyPEM(buf []byte) (security.PublicKey, error) {
	key, err := internal.LoadPEMPublicKey(bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}
	return publicKeyFromCryptoKey(key)
}

// DecodePublicKeySSH decodes the supplied ssh 'authorized hosts' format
// to obtain a public key.
func DecodePublicKeySSH(buf []byte) (security.PublicKey, error) {
	sshKey, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	if err != nil {
		return nil, err
	}
	key, err := internal.CryptoKeyFromSSHKey(sshKey)
	if err != nil {
		return nil, err
	}
	return publicKeyFromCryptoKey(key)
}

func parseAndMarshalOpenSSHKey(buf []byte) ([]byte, error) {
	pk, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	if err != nil {
		return nil, err
	}
	return pk.Marshal(), nil
}

// SSHSignatureMD5 returns the md5 signature for the supplied openssh
// "authorized hosts" format public key. It produces the same output
// as ssh-keygen -l -f <pub-key-file> -E md5
func SSHSignatureMD5(buf []byte) (string, error) {
	raw, err := parseAndMarshalOpenSSHKey(buf)
	if err != nil {
		return "", err
	}
	return md5Signature(raw), nil
}

// SSHSignatureSHA256 returns the base64 raw encoding sha256 signature for
// the supplied openssh "authorized hosts" format public key. It produces
// the same output as ssh-keygen -l -f <pub-key-file> -E sha256.
func SSHSignatureSHA256(buf []byte) (string, error) {
	raw, err := parseAndMarshalOpenSSHKey(buf)
	if err != nil {
		return "", err
	}
	return sha256Signature(raw), nil
}

func md5Signature(bytes []byte) string {
	const hextable = "0123456789abcdef"
	hash := md5.Sum(bytes)
	var repr [md5.Size * 3]byte
	for i, v := range hash {
		repr[i*3] = hextable[v>>4]
		repr[i*3+1] = hextable[v&0x0f]
		repr[i*3+2] = ':'
	}
	return string(repr[:len(repr)-1])
}

func sha256Signature(bytes []byte) string {
	hash := sha256.Sum256(bytes)
	return base64.RawStdEncoding.EncodeToString(hash[:])
}
