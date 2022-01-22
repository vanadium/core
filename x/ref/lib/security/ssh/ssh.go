// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ssh provides support for using ssh keys and agents with Vanadium
// principals.
package ssh

import (
	"crypto"

	"golang.org/x/crypto/ssh"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/signing/sshagent"
)

// AgentHostedKey represents a private key hosted by an ssh agent. The public
// key file must be accessible and is used to identify the private key hosted
// by the ssh agent. Currently ecdsa and ed25519 key types are supported.
type AgentHostedKey struct {
	PublicKeyFile string
	PublicKey     ssh.PublicKey
	Comment       string
	Agent         *sshagent.Client
}

// NewAgentHostedKey creates a connection to the users ssh agent
// in order to use the private key corresponding to the supplied
// public for signing operations. Thus allowing the use of ssh keys
// without having to separately manage them.
func NewAgentHostedKey(publicKeyFile string) (crypto.PrivateKey, error) {
	key, comment, err := internal.LoadSSHPublicKeyFile(publicKeyFile)
	if err != nil {
		return nil, err
	}
	return &AgentHostedKey{
		PublicKeyFile: publicKeyFile,
		PublicKey:     key,
		Comment:       comment,
		Agent:         sshagent.NewClient(),
	}, nil
}
