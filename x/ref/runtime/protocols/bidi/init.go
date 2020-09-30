// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bidi

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
)

const Name = "bidi"

func init() {
	flow.RegisterProtocol(Name, Bidi{})
}

// Bidi protocol represents the protocol to make a bidirectional RPC. Dial, Resolve, and Listen
// all fail on the Bidi protocol because the RoutingID of the end server must already be in the
// flow.Manager's cache for the bidirectional call to succeed.
type Bidi struct{}

func (Bidi) Dial(ctx *context.T, network, address string, timeout time.Duration) (flow.Conn, error) {
	return nil, ErrorfBidiRoutingIdNotCached(ctx, "bidi routing id not in cache")
}

func (Bidi) Resolve(ctx *context.T, network, address string) (string, []string, error) {
	return "", nil, ErrorfBidiRoutingIdNotCached(ctx, "bidi routing id not in cache")
}

func (Bidi) Listen(ctx *context.T, network, address string) (flow.Listener, error) {
	return nil, ErrorfCannotListenOnBidi(ctx, "cannot listen on bidi protocol")
}
