// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build nacl

package websocket

import (
	"net"
	"runtime/ppapi"
	"sync"

	"v.io/v23/flow"
)

// Ppapi instance which must be set before the Dial is called.
var PpapiInstance ppapi.Instance

func WebsocketConn(address string, ws *ppapi.WebsocketConn) flow.Conn {
	return &wrappedConn{
		address: address,
		ws:      ws,
	}
}

type wrappedConn struct {
	address   string
	ws        *ppapi.WebsocketConn
	readLock  sync.Mutex
	writeLock sync.Mutex
}

func (c *wrappedConn) ReadMsg() ([]byte, error) {
	defer c.readLock.Unlock()
	c.readLock.Lock()
	return c.ws.ReceiveMessage()
}

func (c *wrappedConn) WriteMsg(bufs ...[]byte) (int, error) {
	defer c.writeLock.Unlock()
	c.writeLock.Lock()
	var b []byte
	for _, buf := range bufs {
		b = append(b, buf...)
	}
	if err := c.ws.SendMessage(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wrappedConn) Close() error {
	// TODO(mattr): Sometimes it seems that calling close on a websocket from
	// browsper can hang for up to a minute.  I don't understand why this happens.
	// This is a hack and should be replaced with a real fix at some point.
	go c.ws.Close()
	return nil
}

func (c *wrappedConn) LocalAddr() net.Addr {
	return websocketAddr{s: c.address}
}

type websocketAddr struct {
	s string
}

func (websocketAddr) Network() string {
	return "ws"
}

func (w websocketAddr) String() string {
	return w.s
}
