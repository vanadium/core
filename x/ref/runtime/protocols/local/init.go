// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package local

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
)

func init() {
	flow.RegisterProtocol("local", &local{listeners: map[addr]*listener{}})
}

type addr string

func (a addr) Network() string { return "local" }
func (a addr) String() string  { return string(a) }

type conn struct {
	addr     addr
	incoming chan []byte
	peer     *conn
	*wire
}

func (c *conn) LocalAddr() net.Addr  { return c.addr }
func (c *conn) RemoteAddr() net.Addr { return c.peer.addr }
func (c *conn) ReadMsg() ([]byte, error) {
	select {
	case msg := <-c.incoming:
		return msg, nil
	case <-c.wire.closech:
		return nil, io.EOF
	}
}
func (c *conn) WriteMsg(data ...[]byte) (int, error) {
	l := 0
	for _, b := range data {
		l += len(b)
	}
	agg := make([]byte, 0, l)
	for _, b := range data {
		agg = append(agg, b...)
	}
	select {
	case c.peer.incoming <- agg:
		return l, nil
	case <-c.wire.closech:
		return 0, io.EOF
	}
}

type wire struct {
	mu      sync.Mutex
	closed  bool
	closech chan struct{}
}

func (w *wire) Close() error {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.closech)
	}
	w.mu.Unlock()
	return nil
}

type listener struct {
	addr    addr
	conns   chan *conn
	closech chan struct{}
	local   *local
	closed  bool // protected by local.mu
}

func (l *listener) Accept(ctx *context.T) (flow.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closech:
		return nil, fmt.Errorf("listener closed")
	}
}
func (l *listener) Addr() net.Addr {
	return l.addr
}

func (l *listener) Close() error {
	l.local.mu.Lock()
	if !l.closed {
		l.closed = true
		close(l.closech)
	}
	l.local.mu.Unlock()
	return nil
}

type local struct {
	mu        sync.Mutex
	next      int
	listeners map[addr]*listener
}

func (l *local) nextAddrLocked() addr {
	ret := strconv.FormatInt(int64(l.next), 10)
	l.next++
	return addr(ret)
}

func (l *local) Dial(ctx *context.T, network, address string, timeout time.Duration) (flow.Conn, error) {
	l.mu.Lock()
	listener := l.listeners[addr(address)]
	daddr, aaddr := l.nextAddrLocked(), l.nextAddrLocked()
	l.mu.Unlock()

	if listener == nil {
		return nil, fmt.Errorf("unreachable")
	}

	w := &wire{closech: make(chan struct{})}
	d := &conn{addr: daddr, incoming: make(chan []byte), wire: w}
	a := &conn{addr: aaddr, incoming: make(chan []byte), wire: w}
	d.peer, a.peer = a, d

	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-t.C:
		return nil, fmt.Errorf("timeout")
	case listener.conns <- a:
		return d, nil
	case <-listener.closech:
		return nil, fmt.Errorf("unreachable")
	}
}

func (l *local) Resolve(ctx *context.T, network, address string) (string, []string, error) {
	return network, []string{address}, nil
}

func (l *local) Listen(ctx *context.T, network, address string) (flow.Listener, error) {
	defer l.mu.Unlock()
	l.mu.Lock()
	a := addr(address)
	if a == "" {
		a = l.nextAddrLocked()
	}
	if _, ok := l.listeners[a]; ok {
		return nil, fmt.Errorf("address in use")
	}
	listener := &listener{
		addr:    a,
		conns:   make(chan *conn, 1),
		closech: make(chan struct{}),
		local:   l,
	}
	l.listeners[a] = listener
	return listener, nil
}
