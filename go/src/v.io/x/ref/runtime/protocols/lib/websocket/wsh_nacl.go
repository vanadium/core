// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build nacl

package websocket

import (
	"net/url"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
)

type WS struct{}

func (WS) Dial(ctx *context.T, protocol, address string, timeout time.Duration) (flow.Conn, error) {
	inst := PpapiInstance
	u, err := url.Parse("ws://" + address)
	if err != nil {
		return nil, err
	}
	ws, err := inst.DialWebsocket(u.String())
	if err != nil {
		return nil, err
	}
	return WebsocketConn(address, ws), nil
}

func (WS) Resolve(ctx *context.T, protocol, address string) (string, []string, error) {
	return "ws", []string{address}, nil
}

func (WS) Listen(ctx *context.T, protocol, address string) (flow.Listener, error) {
	return nil, NewErrListenCalledInNaCl(ctx)
}
