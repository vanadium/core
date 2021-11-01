// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sshagent provides the ability to use openssh's ssh-agent
// to carry out key signing operations using keys stored therein.
// This allows ssh keys to be used as Vanadium principals.
package sshagent

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"v.io/v23/security"
	secinternal "v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/signing"
	"v.io/x/ref/lib/security/signing/internal"
)

// Client represents an ssh-agent client.
type Client struct {
	mu    sync.Mutex
	conn  net.Conn
	agent agent.ExtendedAgent
	sock  string
}

// NewClient returns a new instance of Client.
func NewClient() *Client {
	return &Client{}
}

// NewSigningService returns an implementation of signing.Service that uses
// an ssh-agent to perform signing operations.
func NewSigningService() signing.Service {
	return &Client{}
}

// SetAgentSockName will call the SetAgentSockName method on the
// specified signing.Service iff it is an instance of sshagent.Client.
func SetAgentSockName(service signing.Service, name string) {
	if ac, ok := service.(*Client); ok {
		ac.SetAgentSockName(name)
	}
}

// SetAgentSockName specifies the name of the unix domain socket to use
// to communicate with the agent. It must be called before any other
// method, otherwise the value of the SSH_AUTH_SOCK environment variable
// is used.
func (ac *Client) SetAgentSockName(name string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.sock = name
}

func (ac *Client) connect() error {
	ac.mu.Lock()
	sockName := ac.sock
	if len(sockName) == 0 {
		sockName = os.Getenv("SSH_AUTH_SOCK")
	}
	if ac.conn != nil && ac.agent != nil {
		ac.mu.Unlock()
		return nil
	}
	ac.mu.Unlock()
	conn, err := net.Dial("unix", sockName)
	if err != nil {
		return fmt.Errorf("failed to open %v: %v", sockName, err)
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.conn = conn
	ac.agent = agent.NewClient(conn)
	return nil
}

// Lock will lock the agent using the specified passphrase.
func (ac *Client) Lock(passphrase []byte) error {
	if err := ac.connect(); err != nil {
		return err
	}
	if err := ac.agent.Lock(passphrase); err != nil {
		return fmt.Errorf("failed to lock agent: %v", err)
	}
	return nil
}

// Unlock will unlock the agent using the specified passphrase.
func (ac *Client) Unlock(passphrase []byte) error {
	if err := ac.connect(); err != nil {
		return err
	}
	if err := ac.agent.Unlock(passphrase); err != nil {
		return fmt.Errorf("failed to unlock agent: %v", err)
	}
	return nil
}

func handleLock(client agent.ExtendedAgent, pw []byte) (func(err error) error, error) {
	passthrough := func(err error) error { return err }
	if pw == nil {
		return passthrough, nil
	}
	if err := client.Unlock(pw); err != nil {
		return passthrough, err
	}
	return func(err error) error {
		nerr := client.Lock(pw)
		if err == nil {
			err = nerr
		}
		return err
	}, nil
}

// Signer implements signing.Service.
func (ac *Client) Signer(ctx context.Context, publicKeyFile string, passphrase []byte) (s security.Signer, err error) {
	key, comment, err := secinternal.LoadSSHPublicKeyFile(publicKeyFile)
	if err != nil {
		return nil, err
	}
	if err := ac.connect(); err != nil {
		return nil, err
	}

	relock, err := handleLock(ac.agent, passphrase)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = relock(err)
	}()

	k, err := ac.lookup(key, comment)
	if err != nil {
		return nil, err
	}
	pk, err := ssh.ParsePublicKey(k.Marshal())
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key for %v: %v", key, err)
	}
	var vpk security.PublicKey
	var impl signImpl
	switch pk.Type() {
	case ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521:
		vpk, err = internal.FromECDSAKey(pk)
		impl = ecdsaSign
	case ssh.KeyAlgoED25519:
		vpk, err = internal.FromED25512Key(pk)
		impl = ed25519Sign
	case ssh.KeyAlgoRSA:
		vpk, err = internal.FromRSAKey(pk)
		impl = rsaSign
	default:
		return nil, fmt.Errorf("unsupported ssh key key type %v", pk.Type())
	}
	if err != nil {
		return nil, err
	}
	return &signer{
		passphrase: passphrase,
		service:    ac,
		sshPK:      pk,
		v23PK:      vpk,
		key:        k,
		name:       ssh.FingerprintSHA256(key),
		impl:       impl,
	}, nil
}

// Close implements signing.Service.
func (ac *Client) Close(ctx context.Context) error {
	return ac.conn.Close()
}

func (ac *Client) lookup(key ssh.PublicKey, comment string) (*agent.Key, error) {
	keys, err := ac.agent.List()
	if err != nil {
		return nil, err
	}
	pk := key.Marshal()
	for _, key := range keys {
		if bytes.Equal(pk, key.Blob) && key.Comment == comment {
			if !internal.IsSupported(key) {
				return nil, fmt.Errorf("key %v (%v) is not one of the supported type types: %v", comment, key.Type(), strings.Join(internal.SupportedKeyTypes(), ", "))
			}
			return key, nil
		}
	}
	return nil, fmt.Errorf("key not found: %v %v ", ssh.FingerprintSHA256(key), comment)
}

func ecdsaSign(ac *Client, sshPK ssh.PublicKey, v23PK, purpose, message []byte, name string) (security.Signature, error) {
	digest, digestType, err := internal.DigestsForSSH(sshPK, v23PK, purpose, message)
	if err != nil {
		return security.Signature{}, fmt.Errorf("failed to generate message digesT: %v", err)
	}
	sig, err := ac.agent.Sign(sshPK, digest)
	if err != nil {
		return security.Signature{}, fmt.Errorf("signature operation failed for %v: %v", name, err)
	}
	var ecSig struct {
		R, S *big.Int
	}
	if err := ssh.Unmarshal(sig.Blob, &ecSig); err != nil {
		return security.Signature{}, fmt.Errorf("failed to unmarshal ECDSA signature: %v", err)
	}
	return security.Signature{
		Purpose: purpose,
		Hash:    digestType,
		R:       ecSig.R.Bytes(),
		S:       ecSig.S.Bytes(),
	}, nil
}

func ed25519Sign(ac *Client, sshPK ssh.PublicKey, v23PK, purpose, message []byte, name string) (security.Signature, error) {
	digest, digestType, err := internal.HashedDigestsForSSH(sshPK, v23PK, purpose, message)
	if err != nil {
		return security.Signature{}, fmt.Errorf("failed to generate message digesT: %v", err)
	}
	sig, err := ac.agent.Sign(sshPK, digest)
	if err != nil {
		return security.Signature{}, fmt.Errorf("signature operation failed for %v: %v", name, err)
	}
	return security.Signature{
		Purpose: purpose,
		Hash:    digestType,
		Ed25519: sig.Blob,
	}, nil
}

func rsaSign(ac *Client, sshPK ssh.PublicKey, v23PK, purpose, message []byte, name string) (security.Signature, error) {
	digest, digestType, err := internal.DigestsForSSH(sshPK, v23PK, purpose, message)
	if err != nil {
		return security.Signature{}, fmt.Errorf("failed to generate message digesT: %v", err)
	}
	sig, err := ac.agent.SignWithFlags(sshPK, digest, agent.SignatureFlagRsaSha512)
	if err != nil {
		return security.Signature{}, fmt.Errorf("signature operation failed for %v: %v", name, err)
	}
	return security.Signature{
		Purpose: purpose,
		Hash:    digestType,
		Rsa:     sig.Blob,
	}, nil
}

type signImpl func(ac *Client, sshPK ssh.PublicKey, v23PK, purpose, message []byte, name string) (security.Signature, error)
type signer struct {
	service    *Client
	passphrase []byte
	name       string
	sshPK      ssh.PublicKey
	v23PK      security.PublicKey
	key        *agent.Key
	impl       signImpl
}

// Sign implements security.Signer.
func (sn *signer) Sign(purpose, message []byte) (sig security.Signature, err error) {
	relock, err := handleLock(sn.service.agent, sn.passphrase)
	if err != nil {
		return security.Signature{}, err
	}
	defer func() {
		err = relock(err)
	}()
	keyBytes, err := sn.v23PK.MarshalBinary()
	if err != nil {
		return security.Signature{}, fmt.Errorf("failed to marshal public key: %v", sn.v23PK)
	}
	return sn.impl(sn.service, sn.sshPK, keyBytes, purpose, message, sn.name)
}

// PublicKey implements security.PublicKey.
func (sn *signer) PublicKey() security.PublicKey {
	return sn.v23PK
}
