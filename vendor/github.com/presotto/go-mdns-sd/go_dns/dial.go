// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dns should not be used directly by google3; instead, use net/dns/go/dns.
package dns

import (
	"errors"
	netpkg "net"
	"strings"
)

func parseDialNetwork(net string) (afnet string, proto int, err error, handled bool) {
	i := strings.LastIndex(net, ":")
	if i < 0 { // no colon
		switch net {
		case "tcp", "tcp4", "tcp6":
		case "udp", "udp4", "udp6":
		default:
			return
		}
		return net, 0, nil, true
	}
	return
}

func resolveNetAddr(op, net, addr string) (afnet string, a netpkg.Addr, err error, handled bool) {
	afnet, _, err, ok := parseDialNetwork(net)
	if !ok {
		return
	}
	if err != nil {
		handled = true
		return
	}
	if op == "dial" && addr == "" {
		return
	}
	switch afnet {
	default:
		return
	case "tcp", "tcp4", "tcp6":
		if addr != "" {
			a, err = ResolveTCPAddr(afnet, addr)
		}
	case "udp", "udp4", "udp6":
		if addr != "" {
			a, err = ResolveUDPAddr(afnet, addr)
		}
	case "ip", "ip4", "ip6":
		if addr != "" {
			a, err = ResolveIPAddr(afnet, addr)
		}
	}
	handled = true
	return
}

// Dial is like net.Dial but handles DNS resolution itself instead of
// in package net.
func Dial(net, addr string) (netpkg.Conn, error) {
	_, addri, err, handled := resolveNetAddr("dial", net, addr)
	if !handled {
		return netpkg.Dial(net, addr)
	}
	if err != nil {
		return nil, err
	}
	return dialAddr(net, addr, addri)
}

func dialAddr(net, addr string, addri netpkg.Addr) (c netpkg.Conn, err error) {
	switch ra := addri.(type) {
	case *netpkg.TCPAddr:
		c, err = netpkg.DialTCP(net, nil, ra)
	case *netpkg.UDPAddr:
		c, err = netpkg.DialUDP(net, nil, ra)
	case *netpkg.IPAddr:
		c, err = netpkg.DialIP(net, nil, ra)
	default:
		err = errors.New("Unknown network address type")
	}
	if err != nil {
		return nil, err
	}
	return
}
