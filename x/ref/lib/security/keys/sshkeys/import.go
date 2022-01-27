// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"fmt"

	"golang.org/x/crypto/ssh"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
)

type ImportOption func(o *importOptions)

type importOptions struct {
	keyBytes    []byte
	keyFilename string
	agent       bool
}

func ImportPrivateKeyBytes(keyBytes []byte) ImportOption {
	return func(o *importOptions) {
		o.keyBytes = keyBytes
	}
}

func ImportPrivateKeyFile(filename string) ImportOption {
	return func(o *importOptions) {
		o.keyFilename = filename
	}
}

func ImportUsingAgent(v bool) ImportOption {
	return func(o *importOptions) {
		o.agent = v
	}
}

func MarshalForImport(publicKeyBytes []byte, options ...ImportOption) (importedPublicKeyBytes, importedPrivateKeyBytes []byte, err error) {
	opts := importOptions{}
	for _, fn := range options {
		fn(&opts)
	}

	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey(publicKeyBytes)
	if err != nil {
		return
	}

	if opts.agent {
		hostedKey := NewHostedKey(publicKey, comment)
		privKeyBytes, err := marshalHostedKey(hostedKey, nil)
		if err != nil {
			return nil, nil, err
		}
		return publicKeyBytes, privKeyBytes, nil
	}

	if filename := opts.keyFilename; len(filename) > 0 {
		privKeyBytes, err := indirectkeyfiles.MarshalPrivateKey([]byte(filename))
		if err != nil {
			return nil, nil, err
		}
		return publicKeyBytes, privKeyBytes, nil
	}

	if keyBytes := opts.keyBytes; len(keyBytes) > 0 {
		return publicKeyBytes, keyBytes, nil
	}

	return nil, nil, fmt.Errorf("no options were specified for how to import the private key")
}
