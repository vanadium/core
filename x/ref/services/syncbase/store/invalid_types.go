// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"v.io/v23/verror"
)

// InvalidSnapshot is a Snapshot for which all methods return errors.
type InvalidSnapshot struct {
	SnapshotSpecImpl
	Error error // returned by all methods
}

// InvalidStream is a Stream for which all methods return errors.
type InvalidStream struct {
	Error error // returned by all methods
}

// InvalidTransaction is a Transaction for which all methods return errors.
type InvalidTransaction struct {
	Error error // returned by all methods
}

var (
	_ Snapshot    = (*InvalidSnapshot)(nil)
	_ Stream      = (*InvalidStream)(nil)
	_ Transaction = (*InvalidTransaction)(nil)
)

////////////////////////////////////////////////////////////
// InvalidSnapshot

// Abort implements the store.Snapshot interface.
func (s *InvalidSnapshot) Abort() error {
	return convertError(s.Error)
}

// Get implements the store.StoreReader interface.
func (s *InvalidSnapshot) Get(key, valbuf []byte) ([]byte, error) {
	return valbuf, convertError(s.Error)
}

// Scan implements the store.StoreReader interface.
func (s *InvalidSnapshot) Scan(start, limit []byte) Stream {
	return &InvalidStream{s.Error}
}

////////////////////////////////////////////////////////////
// InvalidStream

// Advance implements the store.Stream interface.
func (s *InvalidStream) Advance() bool {
	return false
}

// Key implements the store.Stream interface.
func (s *InvalidStream) Key(keybuf []byte) []byte {
	panic(s.Error)
}

// Value implements the store.Stream interface.
func (s *InvalidStream) Value(valbuf []byte) []byte {
	panic(s.Error)
}

// Err implements the store.Stream interface.
func (s *InvalidStream) Err() error {
	return convertError(s.Error)
}

// Cancel implements the store.Stream interface.
func (s *InvalidStream) Cancel() {
}

////////////////////////////////////////////////////////////
// InvalidTransaction

// ResetForRetry implements the store.Transaction interface.
func (tx *InvalidTransaction) ResetForRetry() {
	panic(tx.Error)
}

// Get implements the store.StoreReader interface.
func (tx *InvalidTransaction) Get(key, valbuf []byte) ([]byte, error) {
	return valbuf, convertError(tx.Error)
}

// Scan implements the store.StoreReader interface.
func (tx *InvalidTransaction) Scan(start, limit []byte) Stream {
	return &InvalidStream{tx.Error}
}

// Put implements the store.StoreWriter interface.
func (tx *InvalidTransaction) Put(key, value []byte) error {
	return convertError(tx.Error)
}

// Delete implements the store.StoreWriter interface.
func (tx *InvalidTransaction) Delete(key []byte) error {
	return convertError(tx.Error)
}

// Commit implements the store.Transaction interface.
func (tx *InvalidTransaction) Commit() error {
	return convertError(tx.Error)
}

// Abort implements the store.Transaction interface.
func (tx *InvalidTransaction) Abort() error {
	return convertError(tx.Error)
}

////////////////////////////////////////////////////////////
// Internal helpers

func convertError(err error) error {
	return verror.Convert(verror.IDAction{}, nil, err)
}
