// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ssh provides support for using ssh keys and agents with Vanadium
// principals.
package ssh

import (
	"crypto"
	"os"

	"golang.org/x/crypto/ssh"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/signing/sshagent"
)

// DefaultAgentSockNameFunc can be overridden to return the address of a custom
// ssh agent to use instead of the one specified by SSH_AUTH_SOCK. This is
// primarily intended for tests.
var DefaultAgentSockNameFunc = func() string {
	return os.Getenv("SSH_AUTH_SOCK")
}

// AgentHostedKey represents a private key hosted by an ssh agent. The public
// key file must be accessible and is used to identify the private key hosted
// by the ssh agent.
type AgentHostedKey struct {
	key     ssh.PublicKey
	comment string
	agent   *sshagent.Client
}

type PublicKey struct {
	PublicKey ssh.PublicKey
	Comment   string
}

// Public returns the public key.
func (key *AgentHostedKey) Public() crypto.PublicKey {
	return key.key
}

// Comment returns the comment associated with the public key that is
// required to identify it with the keys stored in the ssh agent.
func (key *AgentHostedKey) Comment() string {
	return key.comment
}

// NewAgentHostedKey creates a connection to the users ssh agent
// in order to use the private key corresponding to the supplied
// public for signing operations. Thus allowing the use of ssh keys
// without having to separately manage them.
func NewSSHAgentHostedKey(publicKeyFile string) (crypto.PrivateKey, error) {
	key, comment, err := internal.ParseSSHPublicKeyFile(publicKeyFile)
	if err != nil {
		return nil, err
	}
	return &AgentHostedKey{
		key:     key,
		comment: comment,
		agent:   sshagent.NewClient(),
	}, nil
}
