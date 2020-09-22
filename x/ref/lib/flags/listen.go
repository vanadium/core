// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"fmt"
	"net"
	"strconv"

	"v.io/v23/rpc"
)

// TCPProtocolFlag implements flag.Value to provide validation of the command
// line values passed to it: tcp, tcp4, tcp6, ws, ws4, ws6, wsh, wsh4, and wsh6
// being the only allowed values.
type TCPProtocolFlag struct {
	Protocol string
}

// Get implements flag.Getter.
func (t TCPProtocolFlag) Get() interface{} {
	return t.Protocol
}

// Set implements flag.Value.
func (t *TCPProtocolFlag) Set(s string) error {
	switch s {
	case "tcp", "tcp4", "tcp6", "ws", "ws4", "ws6", "wsh", "wsh4", "wsh6":
		t.Protocol = s
		return nil
	default:
		return fmt.Errorf("%v: not a tcp protocol", s)
	}
}

// String implements flag.Value.
func (t TCPProtocolFlag) String() string {
	return t.Protocol
}

// IPHostPortFlag implements flag.Value to provide validation of the
// command line value it is set to. The allowed format is <host>:<port> in
// ip4 and ip6 formats. The host may be specified as a hostname or as an IP
// address (v4 or v6). If a hostname is used and it resolves to multiple IP
// addresses then all of those addresses are stored in IPHostPort.
type IPHostPortFlag struct {
	Address string
	Host    string
	IP      []*net.IPAddr
	Port    string
}

// Get implements flag.Getter.
func (ip IPHostPortFlag) Get() interface{} {
	return ip.String()
}

// Set implements flag.Value.
func (ip *IPHostPortFlag) Set(s string) error {
	if len(s) == 0 {
		ip.Address, ip.Port, ip.Host = "", "", ""
		return nil
	}
	ip.Address = s
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		// no port number in s.
		host = s
		ip.Port = "0"
	} else {
		// have a port in s.
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return fmt.Errorf("failed to parse port number from: %v", s)
		}
		ip.Port = port
	}
	// if len(host) == 0 then we have no host, just a port.
	if len(host) > 0 {
		if addr := net.ParseIP(host); addr == nil {
			// Could be a hostname.
			addrs, err := net.LookupIP(host)
			if err != nil || len(addrs) == 0 {
				return fmt.Errorf("%v: neither an IP address nor a host name: %v", host, err)
			}
			for _, a := range addrs {
				ip.IP = append(ip.IP, &net.IPAddr{IP: a})
			}
			ip.Host = host
		} else {
			ip.IP = []*net.IPAddr{{IP: addr}}
		}
		return nil
	}
	return nil
}

// Implements flag.Value.String
func (ip IPHostPortFlag) String() string {
	if len(ip.Address) == 0 && len(ip.Port) == 0 {
		return ""
	}
	host := ip.Host
	if len(ip.Host) == 0 && ip.IP != nil && len(ip.IP) > 0 {
		// We don't have a hostname, so there should be at most one IP address.
		host = ip.IP[0].String()
	}
	return net.JoinHostPort(host, ip.Port)
}

// HostPortFlag implements flag.Value to provide validation of the
// command line value it is set to. The allowed format is <host>:<port>.
// The host may be specified as a hostname or as an IP
// address (v4 or v6). A DNS hostname is never resolved.
type HostPortFlag struct {
	Host string
	Port string
}

// Get implements flag.Getter.
func (ip HostPortFlag) Get() interface{} {
	return ip.String()
}

// Set implements flag.Value.
func (ip *HostPortFlag) Set(s string) error {
	if len(s) == 0 {
		ip.Port, ip.Host = "", ""
		return nil
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		// no port number in s.
		host = s
		ip.Port = "0"
	} else {
		// have a port in s.
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return fmt.Errorf("failed to parse port number from: %s", s)
		}
		ip.Port = port
	}
	ip.Host = host
	return nil
}

// Implements flag.Value.String
func (ip HostPortFlag) String() string {
	if len(ip.Host) == 0 && len(ip.Port) == 0 {
		return ""
	}
	if len(ip.Port) == 0 || ip.Port == "0" {
		return ip.Host
	}
	return net.JoinHostPort(ip.Host, ip.Port)
}

// IPFlag implements flag.Value in order to provide validation of
// IP addresses in the flag package.
type IPFlag struct{ net.IP }

// Get implements flag.Getter.
func (ip IPFlag) Get() interface{} {
	return ip.IP
}

// Set implements flag.Value.
func (ip *IPFlag) Set(s string) error {
	t := net.ParseIP(s)
	if t == nil {
		return fmt.Errorf("failed to parse %v as an IP address", s)
	}
	ip.IP = t
	return nil
}

// Implements flag.Value.String
func (ip IPFlag) String() string {
	return ip.IP.String()
}

// ListenAddrs is the set of listen addresses captured from the command line.
// ListenAddrs mirrors rpc.ListenAddrs.
type ListenAddrs []struct {
	Protocol, Address string
}

// ListenFlags contains the values of the Listen flag group.
type ListenFlags struct {
	Addrs       ListenAddrs
	Proxy       string             `cmdline:"v23.proxy,,object name of proxy service to use to export services across network boundaries"`
	ProxyPolicy ProxyPolicyFlag    `cmdline:"v23.proxy.policy,,policy for choosing from a set of available proxy instances"`
	ProxyLimit  int                `cmdline:"v23.proxy.limit,,'max number of proxies to connect to when the policy is to connect to all proxies; 0 implies all proxies'"`
	Protocol    tcpProtocolFlagVar `cmdline:"v23.tcp.protocol,,protocol to listen with"`
	Addresses   ipHostPortFlagVar  `cmdline:"v23.tcp.address,,address to listen on"`
}

type ProxyPolicyFlag rpc.ProxyPolicy

func (policy ProxyPolicyFlag) Value() rpc.ProxyPolicy {
	return rpc.ProxyPolicy(policy)
}

func (policy ProxyPolicyFlag) Get() interface{} {
	return policy
}

func (policy ProxyPolicyFlag) String() string {
	return rpc.ProxyPolicy(policy).String()
}

func (policy *ProxyPolicyFlag) Set(s string) error {
	switch s {
	case "first":
		*policy = ProxyPolicyFlag(rpc.UseFirstProxy)
	case "random":
		*policy = ProxyPolicyFlag(rpc.UseRandomProxy)
	case "all":
		*policy = ProxyPolicyFlag(rpc.UseAllProxies)
	default:
		return fmt.Errorf("unsupported proxy policy: %v, must be one of 'first', 'random' or 'all'", s)
	}
	return nil
}

type tcpProtocolFlagVar struct {
	isSet     bool
	validator TCPProtocolFlag
}

// Implements flag.Value.Get
func (proto tcpProtocolFlagVar) Get() interface{} {
	return proto.validator.String()
}

func (proto tcpProtocolFlagVar) String() string {
	return proto.validator.String()
}

// Implements flag.Value.Set
func (proto *tcpProtocolFlagVar) Set(s string) error {
	if err := proto.validator.Set(s); err != nil {
		return err
	}
	proto.isSet = true
	return nil
}

type ipHostPortFlagVar struct {
	isSet     bool
	validator IPHostPortFlag
	flags     *ListenFlags
}

// Implements flag.Value.Get
func (ip ipHostPortFlagVar) Get() interface{} {
	return ip.String()
}

// Implements flag.Value.Set
func (ip *ipHostPortFlagVar) Set(s string) error {
	if err := ip.validator.Set(s); err != nil {
		return err
	}
	a := struct {
		Protocol, Address string
	}{
		ip.flags.Protocol.validator.String(),
		ip.validator.String(),
	}
	for _, t := range ip.flags.Addrs {
		if t.Protocol == a.Protocol && t.Address == a.Address {
			return nil
		}
	}
	ip.flags.Addrs = append(ip.flags.Addrs, a)
	ip.isSet = true
	return nil
}

// Implements flag.Value.String
func (ip ipHostPortFlagVar) String() string {
	s := ""
	if ip.flags == nil {
		return s
	}
	for _, a := range ip.flags.Addrs {
		s += fmt.Sprintf("(%s %s)", a.Protocol, a.Address)
	}
	return s
}
