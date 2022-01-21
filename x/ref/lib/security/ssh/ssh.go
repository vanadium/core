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

// DefaultSSHAgentSockNameFunc can be overridden to return the address of a custom
// ssh agent to use instead of the one specified by SSH_AUTH_SOCK. This is
// primarily intended for tests.
var DefaultAgentSockNameFunc = func() string {
	return os.Getenv("SSH_AUTH_SOCK")
}

// SSHAgentHostedKey represents a private key hosted by an ssh agent. The public
// key file must be accessible and is used to identify the private key hosted
// by the ssh agent.
type AgentHostedKey struct {
	PublicKey PublicKey
	Agent     *sshagent.Client
}

type PublicKey struct {
	PublicKey ssh.PublicKey
	Comment   string
}

func (key *AgentHostedKey) PublicKey() crypto.PublicKey {

	return &PublicKey{
		PublicKey: key.PublicKey,
		Comment:   key.Comment,
	}
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
	return &SSHAgentHostedKey{
		PublicKeyFile: publicKeyFile,
		PublicKey:     key,
		Comment:       comment,
		Agent:         sshagent.NewClient(),
	}, nil
}
