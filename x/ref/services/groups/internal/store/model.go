// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

// Store is a key-value store that uses versions for optimistic concurrency
// control. The versions passed to Update and Delete must come from Get. If in
// the meantime some client has called Update or Delete on the same key, the
// version will be stale and the method call will fail.
//
// Note, this API disallows empty versions to simplify implementation. The group
// server is the only client of this API and always specifies versions.
type Store interface {
	// Get returns the value for the given key (decoding into v).
	// Fails if the given key is unknown (ErrUnknownKey).
	Get(k string, v interface{}) (version string, err error)

	// Insert writes the given value for the given key.
	// Fails if an entry already exists for the given key (ErrKeyExists).
	Insert(k string, v interface{}) error

	// Update writes the given value for the given key.
	// Fails if the given key is unknown (ErrUnknownKey).
	// Fails if version doesn't match (ErrBadVersion).
	Update(k string, v interface{}, version string) error

	// Delete deletes the entry for the given key.
	// Fails if the given key is unknown (ErrUnknownKey).
	// Fails if version doesn't match (ErrBadVersion).
	Delete(k string, version string) error

	// Close closes the store. All subsequent method calls will fail.
	Close() error
}
