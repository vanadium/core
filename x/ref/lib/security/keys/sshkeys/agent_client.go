// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"v.io/v23/security"
)

// Client represents an ssh agent client.
type Client struct {
	mu    sync.Mutex
	conn  net.Conn
	agent agent.ExtendedAgent
}

// NewClient returns a new instance of Client.
func NewClient() *Client {
	return &Client{}
}

func (ac *Client) connect(ctx context.Context) error {
	sockName := AgentSocketName(ctx)
	ac.mu.Lock()
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

// Lock will lock the agent using the specified passphrase. Note that the
// passphrase is not zeroed on return.
func (ac *Client) Lock(ctx context.Context, passphrase []byte) error {
	if err := ac.connect(ctx); err != nil {
		return err
	}
	if err := ac.agent.Lock(passphrase); err != nil {
		return fmt.Errorf("failed to lock agent: %v", err)
	}
	return nil
}

// Unlock will unlock the agent using the specified passphrase.
func (ac *Client) Unlock(ctx context.Context, passphrase []byte) error {
	if err := ac.connect(ctx); err != nil {
		return err
	}
	if err := ac.agent.Unlock(passphrase); err != nil {
		return fmt.Errorf("failed to unlock agent: %v", err)
	}
	return nil
}

func relockIfUnlocked(client agent.ExtendedAgent, pw []byte) bool {
	if err := client.Lock(pw); err != nil {
		return false
	}
	err := client.Unlock(pw)
	return err == nil
}

func handleLock(client agent.ExtendedAgent, pw []byte) (func(err error) error, error) {
	passthrough := func(err error) error { return err }
	if len(pw) == 0 {
		return passthrough, nil
	}
	wasUnlocked := false
	if err := client.Unlock(pw); err != nil {
		// The unlock may have failed because the agent was already unlocked,
		// so try to lock and then unlock it!
		wasUnlocked = relockIfUnlocked(client, pw)
		if !wasUnlocked {
			return passthrough, err
		}
	}
	if wasUnlocked {
		return passthrough, nil
	}
	return func(err error) error {
		nerr := client.Lock(pw)
		if err == nil {
			err = nerr
		}
		return err
	}, nil
}

// Signer creates a new security.Signer for a private key that's hosted in the
// ssh agent.
func (ac *Client) Signer(ctx context.Context, hostedKey *HostedKey) (sig security.Signer, err error) {
	if err := ac.connect(ctx); err != nil {
		return nil, err
	}
	relock, err := handleLock(ac.agent, hostedKey.passphrase)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = relock(err)
	}()

	k, err := ac.lookup(hostedKey.publicKey)
	if err != nil {
		return nil, err
	}
	pk, err := ssh.ParsePublicKey(k.Marshal())
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key for %v: %v", hostedKey.publicKey, err)
	}
	var vpk security.PublicKey
	var impl signImpl
	switch pk.Type() {
	case ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521:
		vpk, err = fromECDSAKey(pk)
		impl = ecdsaSign
	case ssh.KeyAlgoED25519:
		vpk, err = fromED25512Key(pk)
		impl = ed25519Sign
	case ssh.KeyAlgoRSA:
		vpk, err = fromRSAKey(pk)
		impl = rsaSign
	default:
		return nil, fmt.Errorf("unsupported ssh key key type %v", pk.Type())
	}
	if err != nil {
		return nil, err
	}
	s := &signer{
		service:   ac,
		hostedKey: hostedKey,
		v23PK:     vpk,
		key:       k,
		name:      ssh.FingerprintSHA256(hostedKey.publicKey),
		impl:      impl,
	}
	return s, nil
}

// Close implements signing.Service.
func (ac *Client) Close(ctx context.Context) error {
	if ac.conn == nil {
		return nil
	}
	return ac.conn.Close()
}

func (ac *Client) lookup(key ssh.PublicKey) (*agent.Key, error) {
	keys, err := ac.agent.List()
	if err != nil {
		return nil, err
	}
	pk := key.Marshal()
	for _, key := range keys {
		if bytes.Equal(pk, key.Blob) {
			if !isSupported(key) {
				return nil, fmt.Errorf("key %v (%v) is not one of the supported type types: %v", ssh.FingerprintSHA256(key), key.Type(), strings.Join(supportedKeyTypes(), ", "))
			}
			return key, nil
		}
	}
	return nil, fmt.Errorf("key not found in ssh agent: %v ", ssh.FingerprintSHA256(key))
}

func ecdsaSign(sn *signer, v23PK, purpose, message []byte) (security.Signature, error) {
	digest, digestType, err := digestsForSSH(sn.hostedKey.publicKey, v23PK, purpose, message)
	if err != nil {
		return security.Signature{}, fmt.Errorf("failed to generate message digesT: %v", err)
	}
	sig, err := sn.service.agent.Sign(sn.hostedKey.publicKey, digest)
	if err != nil {
		return security.Signature{}, fmt.Errorf("signature operation failed for %v: %v", sn.name, err)
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

func ed25519Sign(sn *signer, v23PK, purpose, message []byte) (security.Signature, error) {
	digest, digestType, err := hashedDigestsForSSH(sn.hostedKey.publicKey, v23PK, purpose, message)
	if err != nil {
		return security.Signature{}, fmt.Errorf("failed to generate message digesT: %v", err)
	}
	sig, err := sn.service.agent.Sign(sn.hostedKey.publicKey, digest)
	if err != nil {
		return security.Signature{}, fmt.Errorf("signature operation failed for %v: %v", sn.name, err)
	}
	return security.Signature{
		Purpose: purpose,
		Hash:    digestType,
		Ed25519: sig.Blob,
	}, nil
}

func rsaSign(sn *signer, v23PK, purpose, message []byte) (security.Signature, error) {
	digest, digestType, err := digestsForSSH(sn.hostedKey.publicKey, v23PK, purpose, message)
	if err != nil {
		return security.Signature{}, fmt.Errorf("failed to generate message digesT: %v", err)
	}
	sig, err := sn.service.agent.SignWithFlags(sn.hostedKey.publicKey, digest, agent.SignatureFlagRsaSha512)
	if err != nil {
		return security.Signature{}, fmt.Errorf("signature operation failed for %v: %v", sn.name, err)
	}
	return security.Signature{
		Purpose: purpose,
		Hash:    digestType,
		Rsa:     sig.Blob,
	}, nil
}

type signImpl func(sn *signer, v23PK, purpose, message []byte) (security.Signature, error)

type signer struct {
	service   *Client
	name      string
	hostedKey *HostedKey
	v23PK     security.PublicKey
	key       *agent.Key
	impl      signImpl
}

// Sign implements security.Signer.
func (sn *signer) Sign(purpose, message []byte) (sig security.Signature, err error) {
	relock, err := handleLock(sn.service.agent, sn.hostedKey.passphrase)
	if err != nil {
		return
	}
	defer func() {
		err = relock(err)
	}()
	keyBytes, err := sn.v23PK.MarshalBinary()
	if err != nil {
		err = fmt.Errorf("failed to marshal public key: %v", sn.v23PK)
		return
	}
	sig, err = sn.impl(sn, keyBytes, purpose, message)
	return
}

// PublicKey implements security.PublicKey.
func (sn *signer) PublicKey() security.PublicKey {
	return sn.v23PK
}
