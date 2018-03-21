// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package mdns

import (
	"errors"
	"net"
)

// SetMulticastTTL sets the TTL on packets from this connection.
func SetMulticastTTL(conn *net.UDPConn, ipversion int, v int) error {
	return errors.New("not implemented")
}

func setIPv4MulticastLoopback(fd int, v bool) error {
	return errors.New("not implemented")
}

func setIPv6MulticastLoopback(fd int, v bool) error {
	return errors.New("not implemented")
}
