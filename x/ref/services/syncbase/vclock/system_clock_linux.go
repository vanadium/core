// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Linux-specific bits of the system clock implementation.

package vclock

import (
	"syscall"
	"time"
)

// Linux stores elapsed time in /proc/uptime as seconds since boot, with up to
// two decimal points of precision.
// NOTE(jlodhia): Go's syscall.Sysinfo() returns elapsed time in seconds
// (rounded to the nearest second). Beware.
func (*realSystemClock) ElapsedTime() (time.Duration, error) {
	var sysInfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysInfo); err != nil {
		return 0, err
	}
	return time.Duration(sysInfo.Uptime) * time.Second, nil
}
