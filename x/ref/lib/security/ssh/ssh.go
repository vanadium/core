// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ssh provides support for using ssh keys and agents with Vanadium
// principals.
package ssh

import (
	"bytes"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

const rfc4716PEMType = "SSH2 Public Key"

var rfcSSH2Header = []byte("---- " + rfc4716PEMType + " ----")

// ParsePublicKey will parse an the data as either SSH authorized key
// or in SSH2 (RFC 4716) format.
func ParsePublicKey(data []byte) (ssh.PublicKey, string, error) {
	if bytes.HasPrefix(data, rfcSSH2Header) {
		return ParseRFC4716PublicKey(data)
	}
	return ParseAuthorizedKey(data)
}

// ParseAuthorizedKey will parse an ssh authorized key to extract the
// the public key and comment. All other headers are ignored.
func ParseAuthorizedKey(data []byte) (ssh.PublicKey, string, error) {
	key, comment, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, "", err
	}
	return key, comment, nil
}

// ParseRFC4716PublicKey will parse an SSH2 (RFC 4716) PEM block to extract
// the public key and comment. All other headers are ignored.
func ParseRFC4716PublicKey(data []byte) (ssh.PublicKey, string, error) {
	pemBlock, _ := pem.Decode(data)
	if pemBlock == nil {
		return nil, "", fmt.Errorf("no PEM key block read")
	}
	if pemBlock.Type != rfc4716PEMType {
		return nil, "", fmt.Errorf("no %v PEM block found", rfc4716PEMType)
	}
	comment, ok := pemBlock.Headers["Comment"]
	if !ok {
		return nil, "", fmt.Errorf("no Comment field", rfc4716PEMType)
	}
	keyBytes := make([]byte, base64.StdEncoding.DecodedLen(len(pemBlock.Bytes)))
	n, err := base64.StdEncoding.Decode(keyBytes, pemBlock.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to base64 decode the key: %v", err)
	}
	keyBytes = keyBytes[:n]
	key, err := ssh.ParsePublicKey(keyBytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse key with Comment: %v: %v", comment, err)
	}
	return key, comment, err
}

// MarshalAuthorizedKey marshals the supplied ssh public key with in
// authorized key format and also appends the comment.
func MarshalAuthorizedKey(key ssh.PublicKey, comment string) []byte {
	data := ssh.MarshalAuthorizedKey(key)
	buf := make([]byte, len(data)-1, len(data)+1+len(comment))
	copy(buf, data[:len(data)-1])
	buf = append(buf, ' ')
	buf = append(buf, []byte(comment)...)
	return buf
}
