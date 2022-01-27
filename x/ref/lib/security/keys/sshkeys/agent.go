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
	agentPassphraseKey internalKey = iota
	agentSockNameKey
)

func WithAgentPassphrase(ctx context.Context, passphrase []byte) context.Context {
	return context.WithValue(ctx, agentPassphraseKey, passphrase)
}

func AgentPassphrase(ctx context.Context) []byte {
	if passphrase := ctx.Value(agentPassphraseKey); passphrase != nil {
		return passphrase.([]byte)
	}
	return nil
}

func WithAgentSocketName(ctx context.Context, socketName string) context.Context {
	return context.WithValue(ctx, agentSockNameKey, socketName)
}

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

func (hk *HostedKey) Signer(ctx context.Context) (security.Signer, error) {
	return hk.agent.Signer(ctx, hk.publicKey, AgentPassphrase(ctx))
}

// PublicKey returns the ssh.PublicKey associated with this sshagent hosted key.
func (hk *HostedKey) PublicKey() ssh.PublicKey {
	return hk.publicKey
}
