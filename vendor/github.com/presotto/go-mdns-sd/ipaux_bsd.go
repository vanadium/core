// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd netbsd openbsd

package mdns

import (
	"syscall"
)

func setIPv4MulticastLoopback(fd int, v bool) error {
	return syscall.SetsockoptByte(fd, syscall.IPPROTO_IP, syscall.IP_MULTICAST_LOOP, byte(boolint(v)))
}
