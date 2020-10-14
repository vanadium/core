// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"testing"

	"v.io/v23/context"
	"v.io/v23/logging"
	"v.io/x/ref/internal"
	"v.io/x/ref/internal/logger"
)

func TestManager(t *testing.T) {
	// Make sure that logger implements ManagedLogger.
	global := logger.Global()
	if _, ok := global.(internal.ManagedLogger); !ok {
		t.Fatalf("global logger does not implement logging")
	}
	ctx, _ := context.RootContext()
	ctx = context.WithLogger(ctx, logger.Global())

	// Make sure context.T implements logging.T
	var _ logging.Logger = ctx
}
