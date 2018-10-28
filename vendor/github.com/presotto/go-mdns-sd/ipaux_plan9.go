// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mdns

import (
	"syscall"
)

func setIPv4MulticastLoopback(fd int, v bool) error {
	return syscall.EPLAN9
}

func setIPv6MulticastLoopback(fd int, v bool) error {
	return syscall.EPLAN9
}
