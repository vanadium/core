// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mdns

// Helper routines for manipulating ip connections.

import (
	"net"
)

func boolint(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SetMulticastLoopback turns on or off multicast loopbacks on the interface the connection is on.
func SetMulticastLoopback(conn *net.UDPConn, ipversion int, v bool) error {
	file, err := conn.File()
	if err != nil {
		return err
	}
	fd := int(file.Fd())
	switch ipversion {
	default:
		return setIPv4MulticastLoopback(fd, v)
	case 6:
		return setIPv6MulticastLoopback(fd, v)
	}
}
