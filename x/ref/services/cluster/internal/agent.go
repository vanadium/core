// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"crypto/rand"
	"encoding/hex"
	"reflect"

	"v.io/v23/security"
	"v.io/v23/verror"
)

const secretSize = 256 // Bytes

// The interface for storing secrets and their associated blessings.
type AgentStorage interface {
	// Get retrieves the blessings associated with secret. It returns an
	// error if the secret doesn't exist.
	Get(secret string) (blessings security.Blessings, err error)
	// Put stores the blessings and associates them with secret. It
	// returns an error if the secret already exists.
	Put(secret string, blessings security.Blessings) (err error)
	// Delete deletes the secret and its associated blessings. It returns
	// an error is there was a problem deleting the secret.
	Delete(secret string) error
}

func NewAgent(principal security.Principal, storage AgentStorage) *ClusterAgent {
	return &ClusterAgent{principal, storage}
}

// The Cluster Agent keeps a list of Secret Keys and Blessings associated with
// them. It issues new Blessings when presented with a valid Secret Key. The new
// Blessings are extensions of the Blessings associated with the Secret Key.
type ClusterAgent struct {
	principal security.Principal
	storage   AgentStorage
}

// NewSecret creates a new secret key and associates it with the given
// blessings. The blessings must be bound to the principal of the Cluster
// Agent.
func (a *ClusterAgent) NewSecret(blessings security.Blessings) (string, error) {
	if !reflect.DeepEqual(a.principal.PublicKey(), blessings.PublicKey()) {
		return "", verror.New(verror.ErrBadArg, nil, "blessings bound to wrong PublicKey")
	}
	rawSecret := make([]byte, secretSize)
	if _, err := rand.Read(rawSecret); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(rawSecret)
	if err := a.storage.Put(secret, blessings); err != nil {
		return "", err
	}
	return secret, nil
}

// ForgetSecret forgets the secret key and its associated blessings.
func (a *ClusterAgent) ForgetSecret(secret string) error {
	return a.storage.Delete(secret)
}

// Bless creates new blessing extensions using the blessings associated with the
// given secret key.
func (a *ClusterAgent) Bless(secret string, publicKey security.PublicKey, name string) (security.Blessings, error) {
	blessings, err := a.storage.Get(secret)
	if err != nil {
		return blessings, err
	}
	// blessings was provided to the ClusterAgent via NewSecret and
	// any caveats should have been placed by the caller of NewSecret.
	// The ClusterAgent itself is simply re-delegating its authority to the
	// possessor of 'secret'.
	return a.principal.Bless(publicKey, blessings, name, security.UnconstrainedUse())
}
