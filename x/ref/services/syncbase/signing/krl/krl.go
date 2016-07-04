// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package krl implements a trivial, in-memory key revocation list.
// It is a placeholder for a real key revocation mechanism.
package krl

import "crypto/sha256"
import "time"

// A KRL is a key revocation list.  It maps the hashes of keys that have been revoked
// to revocation times.
type KRL struct {
	table map[[sha256.Size]byte]time.Time
}

var notYetRevoked = time.Now().Add(100 * 365 * 24 * time.Hour) // far future

// New() returns a pointer to a new, empty key recovation list.
func New() *KRL {
	return &KRL{table: make(map[[sha256.Size]byte]time.Time)}
}

// Revoke() inserts an entry into *krl recording that key[] was revoked at time
// "when".
func (krl *KRL) Revoke(key []byte, when time.Time) {
	krl.table[sha256.Sum256(key)] = when
}

// RevocationTime() returns the revocation time for key[].
// If key[] is not in the list, a time in the far future is returned.
func (krl *KRL) RevocationTime(key []byte) (whenRevoked time.Time) {
	var found bool
	if whenRevoked, found = krl.table[sha256.Sum256(key)]; !found {
		whenRevoked = notYetRevoked
	}
	return whenRevoked
}
