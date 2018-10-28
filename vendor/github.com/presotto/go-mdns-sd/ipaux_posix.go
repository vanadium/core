// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd

package mdns

import (
	"net"
	"os"
	"syscall"
)

// SetMulticastTTL sets the TTL on packets from this connection.
func SetMulticastTTL(conn *net.UDPConn, ipversion int, v int) error {
	var proto, opt int
	switch ipversion {
	default:
		proto = syscall.IPPROTO_IP
		opt = syscall.IP_MULTICAST_TTL
	case 6:
		proto = syscall.IPPROTO_IPV6
		opt = syscall.IPV6_MULTICAST_HOPS
	}
	if file, err := conn.File(); err == nil {
		fd := int(file.Fd())
		err := syscall.SetsockoptInt(fd, proto, opt, v)
		if err != nil {
			return os.NewSyscallError("setsockopt", err)
		}
		file.Close()
	} else {
		return err
	}
	return nil
}

func setIPv6MulticastLoopback(fd int, v bool) error {
	return syscall.SetsockoptInt(fd, syscall.IPPROTO_IPV6, syscall.IPV6_MULTICAST_LOOP, boolint(v))
}
