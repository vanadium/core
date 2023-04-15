// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !nacl

package websocket

import (
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/x/ref/runtime/protocols/lib/tcputil"
)

// TODO(jhahn): Figure out a way for this mapping to be shared.
var mapWebSocketToTCP = map[string]string{"ws": "tcp", "ws4": "tcp4", "ws6": "tcp6", "wsh": "tcp", "wsh4": "tcp4", "wsh6": "tcp6", "tcp": "tcp", "tcp4": "tcp4", "tcp6": "tcp6"}

const bufferSize = 4096

type WS struct{}

func (WS) Dial(ctx *context.T, protocol, address string, timeout time.Duration) (flow.Conn, error) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	tcp := mapWebSocketToTCP[protocol]
	conn, err := net.DialTimeout(tcp, address, timeout)
	if err != nil {
		return nil, err
	}
	conn.SetReadDeadline(deadline) //nolint:errcheck
	if err := tcputil.EnableTCPKeepAlive(conn); err != nil {
		return nil, err
	}
	u, err := url.Parse("ws://" + address)
	if err != nil {
		return nil, err
	}

	dialer := &websocket.Dialer{
		ReadBufferSize:  bufferSize,
		WriteBufferSize: bufferSize,
		NetDial: func(net, addr string) (net.Conn, error) {
			return conn, nil
		},
	}
	ws, resp, err := dialer.DialContext(ctx, u.String(), http.Header{})
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	var zero time.Time
	conn.SetDeadline(zero) //nolint:errcheck
	return WebsocketConn(ws), nil
}

func (WS) Resolve(ctx *context.T, protocol, address string) (string, []string, error) {
	addrs, err := tcputil.TCPResolveAddrs(ctx, address)
	return "ws", addrs, err
}

func (WS) Listen(ctx *context.T, protocol, address string) (flow.Listener, error) {
	return listener(protocol, address, false)
}
