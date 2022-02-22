// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"context"
	"os"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
)

type internalKey int

const (
	agentSockNameKey internalKey = iota
)

// WithAgentSocketName returns a context with the specified socket name. This is
// primarily intended for tests.
func WithAgentSocketName(ctx context.Context, socketName string) context.Context {
	return context.WithValue(ctx, agentSockNameKey, socketName)
}

// AgentSocketName returns the socket name associated with the context or
// the return value of DefaultSockNameFunc() if there is no such socket name.
func AgentSocketName(ctx context.Context) string {
	if sockname := ctx.Value(agentSockNameKey); sockname != nil {
		if sn := sockname.(string); len(sn) > 0 {
			return sn
		}
	}
	return DefaultSockNameFunc()
}

// DefaultSockNameFunc can be overridden to return the address of a custom
// ssh agent to use instead of the one specified by SSH_AUTH_SOCK. This is
// primarily intended for tests.
var DefaultSockNameFunc = func() string {
	return os.Getenv("SSH_AUTH_SOCK")
}

// HostededKey represents a private key hosted by an ssh agent.
type HostedKey struct {
	publicKey  ssh.PublicKey
	comment    string
	passphrase []byte
	agent      *Client
}

// Comment returns the comment associated with the original ssh public key.
func (hk *HostedKey) Comment() string {
	return hk.comment
}

// NewHostedKeyFile calls NewHostedKey with the contents of the specified
// file.
func NewHostedKeyFile(publicKeyFile string, passphrase []byte) (*HostedKey, error) {
	keyBytes, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return nil, err
	}
	key, comment, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
	if err != nil {
		return nil, err
	}
	return NewHostedKey(key, comment, passphrase), nil
}

// NewHostedKey creates a connection to the users ssh agent in order to use the
// private key corresponding to the supplied public for signing operations.
// The passphrase, if supplied, is used to unlock/lock the agent. Note that
// the passphrase for unlocking/locking the agent may also be obtained indirectly
// when the PEM encoding of the private key is parsed via keys.ParsePrivateKey
// for example. The passphrase is not zeroed.
func NewHostedKey(key ssh.PublicKey, comment string, passphrase []byte) *HostedKey {
	return &HostedKey{
		publicKey:  key,
		comment:    comment,
		agent:      NewClient(),
		passphrase: passphrase,
	}
}

// Signer returns a security.Signer that is hosted by an ssh agent. The
// returned signer will retain a copy of any passphrase in ctx and will
// zero that copy when it is itself garbage collected.
func (hk *HostedKey) Signer(ctx context.Context) (security.Signer, error) {
	return hk.agent.Signer(ctx, hk)
}

// PublicKey returns the ssh.PublicKey associated with this sshagent hosted key.
func (hk *HostedKey) PublicKey() ssh.PublicKey {
	return hk.publicKey
}

func (hk *HostedKey) setPassphrase(passphrase []byte) {
	if len(passphrase) == 0 {
		hk.passphrase = nil
		return
	}
	hk.passphrase = passphrase
}
