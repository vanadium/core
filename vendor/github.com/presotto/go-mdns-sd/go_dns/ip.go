// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// netpkg.IP, TCP, UDP socket address lookup

package dns

import (
	netpkg "net"
	"strconv"
)

// These are probed by the real package net.
const (
	supportsIPv6    = true
	supportsIPv4map = true
)

func firstFavoriteAddr(filter func(netpkg.IP) netpkg.IP, addrs []string) (addr netpkg.IP) {
	if filter == nil {
		// We'll take any netpkg.IP address, but since the dialing code
		// does not yet try multiple addresses, prefer to use
		// an netpkg.IPv4 address if possible.  This is especially relevant
		// if localhost resolves to [ipv6-localhost, ipv4-localhost].
		// Too much code assumes localhost == ipv4-localhost.
		addr = firstSupportedAddr(ipv4only, addrs)
		if addr == nil {
			addr = firstSupportedAddr(anyaddr, addrs)
		}
	} else {
		addr = firstSupportedAddr(filter, addrs)
	}
	return
}

func firstSupportedAddr(filter func(netpkg.IP) netpkg.IP, addrs []string) netpkg.IP {
	for _, s := range addrs {
		if addr := filter(netpkg.ParseIP(s)); addr != nil {
			return addr
		}
	}
	return nil
}

func anyaddr(x netpkg.IP) netpkg.IP {
	if x4 := x.To4(); x4 != nil {
		return x4
	}
	if supportsIPv6 {
		return x
	}
	return nil
}

func ipv4only(x netpkg.IP) netpkg.IP { return x.To4() }

func ipv6only(x netpkg.IP) netpkg.IP {
	// Only return addresses that we can use
	// with the kernel's netpkg.IPv6 addressing modes.
	if len(x) == netpkg.IPv6len && x.To4() == nil && supportsIPv6 {
		return x
	}
	return nil
}

// Convert "host:port" into netpkg.IP address and port.
func hostPortToIP(net, hostport string) (ip netpkg.IP, iport int, err error) {
	host, port, err := netpkg.SplitHostPort(hostport)
	if err != nil {
		return nil, 0, err
	}

	var addr netpkg.IP
	if host != "" {
		// Try as an netpkg.IP address.
		addr = netpkg.ParseIP(host)
		if addr == nil {
			var filter func(netpkg.IP) netpkg.IP
			if net != "" && net[len(net)-1] == '4' {
				filter = ipv4only
			}
			if net != "" && net[len(net)-1] == '6' {
				filter = ipv6only
			}
			// Not an netpkg.IP address.  Try as a DNS name.
			addrs, err := LookupHost(host)
			if err != nil {
				return nil, 0, err
			}
			addr = firstFavoriteAddr(filter, addrs)
			if addr == nil {
				// should not happen
				return nil, 0, &netpkg.AddrError{"LookupHost returned no suitable address", addrs[0]}
			}
		}
	}

	p, err := strconv.Atoi(port)
	if err != nil {
		p, err = netpkg.LookupPort(net, port)
		if err != nil {
			return nil, 0, err
		}
	}
	if p < 0 || p > 0xFFFF {
		return nil, 0, &netpkg.AddrError{"invalid port", port}
	}

	return addr, p, nil

}

// Convert "host" into IP address.
func hostToIP(net, host string) (ip netpkg.IP, err error) {
	var addr netpkg.IP
	// Try as an IP address.
	addr = netpkg.ParseIP(host)
	if addr == nil {
		filter := anyaddr
		if net != "" && net[len(net)-1] == '4' {
			filter = ipv4only
		}
		if net != "" && net[len(net)-1] == '6' {
			filter = ipv6only
		}
		// Not an IP address.  Try as a DNS name.
		addrs, err1 := LookupHost(host)
		if err1 != nil {
			err = err1
			goto Error
		}
		addr = firstFavoriteAddr(filter, addrs)
		if addr == nil {
			// should not happen
			err = &netpkg.AddrError{"LookupHost returned no suitable address", addrs[0]}
			goto Error
		}
	}
	return addr, nil
Error:
	return nil, err
}

// ResolveTCPAddr parses addr as a TCP address of the form
// host:port and resolves domain names or port names to
// numeric addresses on the network net, which must be "tcp",
// "tcp4" or "tcp6".  A literal netpkg.IPv6 host address must be
// enclosed in square brackets, as in "[::]:80".
func ResolveTCPAddr(net, addr string) (*netpkg.TCPAddr, error) {
	ip, port, err := hostPortToIP(net, addr)
	if err != nil {
		return nil, err
	}
	return &netpkg.TCPAddr{IP: ip, Port: port}, nil
}

// ResolveUDPAddr parses addr as a UDP address of the form
// host:port and resolves domain names or port names to
// numeric addresses on the network net, which must be "udp",
// "udp4" or "udp6".  A literal netpkg.IPv6 host address must be
// enclosed in square brackets, as in "[::]:80".
func ResolveUDPAddr(net, addr string) (*netpkg.UDPAddr, error) {
	ip, port, err := hostPortToIP(net, addr)
	if err != nil {
		return nil, err
	}
	return &netpkg.UDPAddr{IP: ip, Port: port}, nil
}

// ResolveIPAddr parses addr as a IP address and resolves domain
// names to numeric addresses on the network net, which must be
// "ip", "ip4" or "ip6".  A literal IPv6 host address must be
// enclosed in square brackets, as in "[::]".
func ResolveIPAddr(net, addr string) (*netpkg.IPAddr, error) {
	ip, err := hostToIP(net, addr)
	if err != nil {
		return nil, err
	}
	return &netpkg.IPAddr{IP: ip}, nil
}
