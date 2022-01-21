// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

// ParseSSHPublicKeyFile parses a public key file in SSH authorized hosts format
// or in SSH2 pem format. A file with a suffix .pub is expected to be in
// authorized hosts format, one ending in .pem to be in SSH2 pem format.
func ParseSSHPublicKeyFile(filename string) (ssh.PublicKey, string, error) {
	if !strings.HasSuffix(filename, ".pub") {
		return nil, "", fmt.Errorf("%v does not have suffix: .pub", filename)
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	switch {
	case strings.HasSuffix(filename, ".pub"):
		key, comment, err := loadSSHAuthorizedHosts(f)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load ssh public key from: %v: %v", filename, err)
		}
		return key, comment, nil
	case strings.HasSuffix(filename, ".pem"):
		key, comment, err := loadSSH2PEM(f)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load ssh public key from: %v: %v", filename, err)
		}
		return key, comment, nil
	}
}

// loadSSHAuthorizedHosts loads a public key in SSH authorized hosts format.
func loadSSHAuthorizedHosts(r io.Reader) (ssh.PublicKey, string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read bytes: %v", err)
	}
	key, comment, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, "", err
	}
	return key, comment, nil
}

// loadSSH2PEM loads a public key in SSH2 pem format.
func loadSSH2PEM(r io.Reader) (ssh.PublicKey, string, error) {
	pemBlockBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read bytes: %v", err)
	}
	pemBlock, _ := pem.Decode(pemBlockBytes)
	if pemBlock == nil {
		return nil, "", fmt.Errorf("no PEM key block read")
	}
	if pemBlock.Type != ssh2PEMType {
		return nil, "", fmt.Errorf("no %v PEM block found", ssh2PEMType)
	}
	comment, ok := pemBlock.Head["Comment"]
	if !ok {
		return nil, "", fmt.Errorf("no Comment field", ssh2PEMType)
	}

	keyBytes := make([]byte, base64.StdEncoding.DecodedLen(len(pemBlock.Bytes)))
	n, err := base64.StdEncoding.Decode(key, pemBlock.Bytes)
	if err != nil {
		return nil, "", err
	}
	keyBytes = keyBytes[:n]
	key, err = ssh.ParsePublicKey(keyBytes)
	if err != nil {
		return nil, "", err
	}

}
