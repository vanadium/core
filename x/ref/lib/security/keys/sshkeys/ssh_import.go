// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"golang.org/x/crypto/ssh"
)

// ImportAgentHostedPrivateKeyBytes returns the byte representation for an imported
// ssh public key and associated private key that is hosted in an ssh agent.
// This is essentially a reference to the private key.
func ImportAgentHostedKeyBytes(keyBytes []byte) (publicKeyBytes, privateKeyBytes []byte, err error) {
	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
	if err != nil {
		return nil, nil, err
	}
	hostedKey := NewHostedKey(publicKey, comment, nil)
	privKeyBytes, err := marshalHostedKey(hostedKey, nil)
	if err != nil {
		return nil, nil, err
	}
	return keyBytes, privKeyBytes, nil
}
