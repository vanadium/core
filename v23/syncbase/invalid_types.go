// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

// invalidStream is a Stream for which all methods return errors.
type invalidStream struct {
	err error // returned by all methods
}

var _ Stream = (*invalidStream)(nil)

// Advance implements the Stream interface.
func (_ *invalidStream) Advance() bool {
	return false
}

// Err implements the Stream interface.
func (s *invalidStream) Err() error {
	return s.err
}

// Cancel implements the Stream interface.
func (_ *invalidStream) Cancel() {
}

// invalidScanStream is a ScanStream for which all methods return errors.
type invalidScanStream struct {
	invalidStream
}

var _ ScanStream = (*invalidScanStream)(nil)

// Key implements the ScanStream interface.
func (s *invalidScanStream) Key() string {
	panic(s.err)
}

// Value implements the ScanStream interface.
func (s *invalidScanStream) Value(value interface{}) error {
	panic(s.err)
}

// invalidWatchStream is a WatchStream for which all methods return errors.
type invalidWatchStream struct {
	invalidStream
}

var _ WatchStream = (*invalidWatchStream)(nil)

// Change implements the WatchStream interface.
func (s *invalidWatchStream) Change() WatchChange {
	panic(s.err)
}
