// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package client defines an interface that syncbase clients must implement.
// Each syncbase instance in a longevity test will have an associated client
// that interacts with it.  The clients' behavior should reflect real-world use
// cases.
package client

import (
	"v.io/v23/context"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
)

// Client represents a syncbase client in a longevity test.
type Client interface {
	// String returns a string representation of the client, suitable for
	// printing.
	String() string

	// Start starts the client, which will perform operations on databases
	// corresponding to the given models.  Start must not block.  It is the
	// implementation's responsibility to guarantee that the databases and
	// their collections exist.
	// TODO(nlacasse): Should implementation panic on non-recoverable errors?
	Start(ctx *context.T, sbName string, dbs model.DatabaseSet)

	// Stop stops the client and waits for it to exit.
	Stop() error
}
