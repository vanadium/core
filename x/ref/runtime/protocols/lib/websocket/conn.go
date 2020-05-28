// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !nacl

package websocket

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"v.io/v23/flow"
)

// WebsocketConn provides a flow.Conn interface for a websocket connection.
//nolint:golint // API change required.
func WebsocketConn(ws *websocket.Conn) flow.Conn {
	return &wrappedConn{ws: ws}
}

// wrappedConn provides a flow.Conn interface to a websocket.
// The underlying websocket connection needs regular calls to Read to make sure
// websocket control messages (such as pings) are processed by the websocket
// library.
type wrappedConn struct {
	ws *websocket.Conn

	// The gorilla docs aren't explicit about reading and writing from
	// different goroutines.  It is explicit that only one goroutine can
	// do a write at any given time and only one goroutine can do a read
	// at any given time.  Based on inspection it seems that using a reader
	// and writer simultaneously is safe, but this might change with
	// future changes.  We can't actually share the lock, because this means
	// that we can't write while we are waiting for a message, causing some
	// deadlocks where a write is need to unblock a read.
	writeLock sync.Mutex
	readLock  sync.Mutex
}

func (c *wrappedConn) ReadMsg() ([]byte, error) {
	c.readLock.Lock()
	defer c.readLock.Unlock()
	t, b, err := c.ws.ReadMessage()
	if err != nil {
		return nil, err
	}
	if t != websocket.BinaryMessage {
		return nil, fmt.Errorf("Unexpected message type %d", t)
	}
	return b, nil
}

func (c *wrappedConn) WriteMsg(bufs ...[]byte) (int, error) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	if len(bufs) == 0 {
		return 0, nil
	}
	var b []byte
	for _, buf := range bufs {
		b = append(b, buf...)
	}
	if err := c.ws.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wrappedConn) Close() error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	// Send an EOF control message to the remote end so that it can
	// handle the close gracefully.
	msg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "EOF")
	//nolint:errcheck
	c.ws.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
	return c.ws.Close()
}

func (c *wrappedConn) LocalAddr() net.Addr {
	return c.ws.LocalAddr()
}

func (c *wrappedConn) RemoteAddr() net.Addr {
	return c.ws.RemoteAddr()
}

// hybridConn is used by the 'hybrid' protocol that can accept
// either 'tcp' or 'websocket' connections. In particular, it allows
// for the reader to peek and buffer the first n bytes of a stream
// in order to determine what the connection type is.
type hybridConn struct {
	conn     net.Conn
	buffered []byte
}

func (wc *hybridConn) Read(b []byte) (int, error) {
	lbuf := len(wc.buffered)
	if lbuf == 0 {
		return wc.conn.Read(b)
	}
	copyn := copy(b, wc.buffered)
	wc.buffered = wc.buffered[copyn:]
	if len(b) > copyn {
		n, err := wc.conn.Read(b[copyn:])
		return copyn + n, err
	}
	return copyn, nil
}

func (wc *hybridConn) Write(b []byte) (n int, err error) {
	return wc.conn.Write(b)
}

func (wc *hybridConn) Close() error {
	return wc.conn.Close()
}

func (wc *hybridConn) LocalAddr() net.Addr {
	return &addr{"wsh", wc.conn.LocalAddr().String()}
}

func (wc *hybridConn) RemoteAddr() net.Addr {
	return &addr{"wsh", wc.conn.RemoteAddr().String()}
}

func (wc *hybridConn) SetDeadline(t time.Time) error {
	return wc.conn.SetDeadline(t)
}

func (wc *hybridConn) SetReadDeadline(t time.Time) error {
	return wc.conn.SetReadDeadline(t)
}

func (wc *hybridConn) SetWriteDeadline(t time.Time) error {
	return wc.conn.SetWriteDeadline(t)
}
