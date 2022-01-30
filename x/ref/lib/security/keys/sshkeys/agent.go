// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"context"
	"os"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
)

type internalKey int

const (
	agentPassphraseKey internalKey = iota
	agentSockNameKey
)

// WithAgentPassphrase returns a context with the specified passphrase that is
// used to lock the ssh agent. The passphrase will be zero'ed after when
// read by AgentPassphrase.
func WithAgentPassphrase(ctx context.Context, passphrase []byte) context.Context {
	return context.WithValue(ctx, agentPassphraseKey, passphrase)
}

// AgentPassphrase returns a copy of passphrase associated with this context
// and then ovewrites that passphrase with zeros.
func AgentPassphrase(ctx context.Context) []byte {
	if passphrase := ctx.Value(agentPassphraseKey); passphrase != nil {
		pp := passphrase.([]byte)
		if len(pp) == 0 {
			return nil
		}
		cpy := make([]byte, len(pp))
		copy(cpy, pp)
		keys.ZeroPassphrase(pp)
		return cpy
	}
	return nil
}

// WithAgentSocketName returns a context with the specified socket name. This is
// primarily intended for tests.
func WithAgentSocketName(ctx context.Context, socketName string) context.Context {
	return context.WithValue(ctx, agentSockNameKey, socketName)
}

// AgentSocketName returns the socket name associated with the context or
// the return value of DefaultSockNameFunc() if there is no such socket name.
func AgentSocketName(ctx context.Context) string {
	if sockname := ctx.Value(agentSockNameKey); sockname != nil {
		return sockname.(string)
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
	publicKey ssh.PublicKey
	comment   string
	agent     *Client
}

// Comment returns the comment associated with the original ssh public key.
func (hk *HostedKey) Comment() string {
	return hk.comment
}

// NewHostedKey creates a connection to the users ssh agent
// in order to use the private key corresponding to the supplied
// public for signing operations. Thus allowing the use of ssh keys
// without having to separately manage them.
func NewHostedKey(key ssh.PublicKey, comment string) *HostedKey {
	return &HostedKey{
		publicKey: key,
		comment:   comment,
		agent:     NewClient(),
	}
}

// Signer returns a security.Signer that is hosted by an ssh agent.
func (hk *HostedKey) Signer(ctx context.Context) (security.Signer, error) {
	return hk.agent.Signer(ctx, hk.publicKey, AgentPassphrase(ctx))
}

// PublicKey returns the ssh.PublicKey associated with this sshagent hosted key.
func (hk *HostedKey) PublicKey() ssh.PublicKey {
	return hk.publicKey
}
