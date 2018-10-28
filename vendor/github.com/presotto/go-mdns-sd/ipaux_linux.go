// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mdns

import (
	"syscall"
)

func setIPv4MulticastLoopback(fd int, v bool) error {
	return syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_MULTICAST_LOOP, boolint(v))
}
