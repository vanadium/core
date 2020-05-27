// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package tcputil contains functions commonly used to manipulate TCP
// connections.
package tcputil

import (
	"net"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"

	"v.io/x/ref/runtime/protocols/lib/framer"
)

const keepAlivePeriod = 30 * time.Second

// EnableTCPKeepAlive enabled the KeepAlive option on a TCP connection.
//
// Some cloud providers (like Google Compute Engine) blackhole inactive TCP
// connections, we need to set TCP keep alive option to prevent that.
// See: https://developers.google.com/compute/docs/troubleshooting#communicatewithinternet
//
// The same problem can happen when one end of a TCP connection dies and the
// TCP FIN or RST packet doesn't reach the other end, e.g. when the machine
// dies, falls off the network, or when there is packet loss. So, it is best to
// enable this option for all TCP connections.
func EnableTCPKeepAlive(conn net.Conn) error {
	if tcpconn, ok := conn.(*net.TCPConn); ok {
		if err := tcpconn.SetKeepAlivePeriod(keepAlivePeriod); err != nil {
			return err
		}
		return tcpconn.SetKeepAlive(true)
	}
	return nil
}

type TCP struct{}

// Dial dials a net.Conn to a the specific address and adds framing to the connection.
func (TCP) Dial(ctx *context.T, network, address string, timeout time.Duration) (flow.Conn, error) {
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}
	if err := EnableTCPKeepAlive(conn); err != nil {
		return nil, err
	}
	return NewTCPConn(conn), nil
}

// Resolve performs a DNS resolution on the provided network and address.
func (TCP) Resolve(ctx *context.T, network, address string) (string, []string, error) {
	addrs, err := TCPResolveAddrs(ctx, address)
	return network, addrs, err
}

// Listen returns a listener that sets KeepAlive on all accepted connections.
// Connections returned from the listener will be framed.
func (TCP) Listen(ctx *context.T, network, address string) (flow.Listener, error) {
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return &tcpListener{ln}, nil
}

// tcpListener is a wrapper around net.Listener that sets KeepAlive on all
// accepted connections and returns framed flow.Conns.
type tcpListener struct {
	netLn net.Listener
}

func (ln *tcpListener) Accept(ctx *context.T) (flow.Conn, error) {
	conn, err := ln.netLn.Accept()
	if err != nil {
		return nil, err
	}
	if err := EnableTCPKeepAlive(conn); err != nil {
		return nil, err
	}
	return NewTCPConn(conn), nil
}

func (ln *tcpListener) Addr() net.Addr {
	return ln.netLn.Addr()
}

func (ln *tcpListener) Close() error {
	return ln.netLn.Close()
}

func NewTCPConn(c net.Conn) flow.Conn {
	return tcpConn{
		framer.New(c),
		c.LocalAddr(),
		c.RemoteAddr(),
	}
}

type tcpConn struct {
	flow.MsgReadWriteCloser
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (c tcpConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c tcpConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func TCPResolveAddrs(ctx *context.T, address string) ([]string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var addrs []string
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	s := ":" + port
	for _, ip := range ips {
		addrs = append(addrs, ip.String()+s)
	}
	return addrs, nil
}
