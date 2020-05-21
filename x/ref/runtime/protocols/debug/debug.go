// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debug

import (
	"fmt"
	"net"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
)

func init() {
	flow.RegisterProtocol("debug", &debug{})
}

// Filters give you the chance to add your own logic to flow.Conns
// that get created.  This is useful in tests.  For example you might
// want to modify all packets, print them out, or perhaps cause
// errors.
type Filter func(flow.Conn) flow.Conn

type filterKey struct{}

// WithFilter attaches a filter to the context.  Any connection
// accepted or dialed with the debug protocol using this context will
// be wrapped via a call to filter.
func WithFilter(ctx *context.T, filter Filter) *context.T {
	return context.WithValue(ctx, filterKey{}, filter)
}
func filter(ctx *context.T, c flow.Conn) flow.Conn {
	if f, ok := ctx.Value(filterKey{}).(Filter); ok {
		return f(c)
	}
	return c
}

func WrapName(name string) string {
	addr, suffix := naming.SplitAddressName(name)
	if addr == "" {
		return name
	}
	iep, err := naming.ParseEndpoint(addr)
	if err != nil {
		return name
	}
	iep.Protocol, iep.Address = WrapAddress(iep.Protocol, iep.Address)
	return naming.JoinAddressName(iep.String(), suffix)
}

func WrapAddress(protocol, address string) (string, string) {
	return "debug", protocol + "/" + address
}

type addr string

func (a addr) Network() string { return "debug" }
func (a addr) String() string  { return string(a) }

type conn struct {
	base flow.Conn
	addr addr
}

func (c *conn) LocalAddr() net.Addr                  { return c.addr }
func (c *conn) RemoteAddr() net.Addr                 { return c.base.RemoteAddr() }
func (c *conn) ReadMsg() ([]byte, error)             { return c.base.ReadMsg() }
func (c *conn) WriteMsg(data ...[]byte) (int, error) { return c.base.WriteMsg(data...) }
func (c *conn) Close() error                         { return c.base.Close() }
func (c *conn) UnsafeDisableEncryption() bool        { return true }

type listener struct {
	base flow.Listener
	addr addr
}

func (l *listener) Accept(ctx *context.T) (flow.Conn, error) {
	c, err := l.base.Accept(ctx)
	if err != nil {
		return nil, err
	}
	return &conn{base: filter(ctx, c), addr: l.addr}, nil
}
func (l *listener) Addr() net.Addr { return l.addr }
func (l *listener) Close() error   { return l.base.Close() }

type debug struct{}

func (d *debug) Dial(ctx *context.T, network, address string, timeout time.Duration) (flow.Conn, error) {
	var base flow.Protocol
	var baseNetwork string
	if baseNetwork, address, base = baseProtocol(address); base == nil {
		return nil, fmt.Errorf("could not find underlying protocol %q", network)
	}
	if len(baseNetwork) > 0 {
		network = baseNetwork
	}
	c, err := base.Dial(ctx, network, address, timeout)
	if err != nil {
		return nil, err
	}
	return &conn{base: filter(ctx, c), addr: addr(network + "/" + address)}, nil
}
func (d *debug) Listen(ctx *context.T, network, address string) (flow.Listener, error) {
	var base flow.Protocol
	var baseNetwork string
	if baseNetwork, address, base = baseProtocol(address); base == nil {
		return nil, fmt.Errorf("could not find underlying protocol %q", network)
	}
	if len(baseNetwork) > 0 {
		network = baseNetwork
	}
	l, err := base.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return &listener{base: l, addr: addr(l.Addr().Network() + "/" + l.Addr().String())}, nil
}
func (d *debug) Resolve(ctx *context.T, network, address string) (string, []string, error) {
	return network, []string{address}, nil
}

func baseProtocol(in string) (network, address string, base flow.Protocol) {
	parts := strings.SplitN(in, "/", 2)
	if len(parts) == 2 {
		network, address = parts[0], parts[1]
	} else {
		network, address = "", parts[0]
	}
	base, names := flow.RegisteredProtocol(network)
	if network == "" && len(names) > 0 {
		network = names[0]
	}
	return network, address, base
}
