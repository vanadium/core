// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Darwin-specific bits of the system clock implementation.

package vclock

import (
	"bytes"
	"encoding/binary"
	"syscall"
	"time"
	"unsafe"
)

// Darwin provides a "kern.boottime" syscall that returns a Timeval32 object
// with the system's boot time, computed as current time minus the system's
// elapsed time since boot. Thus, subtracting the returned boot time from the
// current time yields the system's elapsed time since boot.
// TODO(sadovsky): Technically, we should make sure the system clock doesn't
// change between the syscall and time.Since(), since both consult the system
// clock.
func (*realSystemClock) ElapsedTime() (time.Duration, error) {
	tv := syscall.Timeval32{}
	if err := sysctlbyname("kern.boottime", &tv); err != nil {
		return 0, err
	}
	return time.Since(time.Unix(int64(tv.Sec), int64(tv.Usec)*1000)), nil
}

// Generic Sysctl buffer unmarshalling.
func sysctlbyname(name string, data interface{}) (err error) {
	val, err := syscall.Sysctl(name)
	if err != nil {
		return err
	}

	buf := []byte(val)

	switch v := data.(type) {
	case *uint64:
		*v = *(*uint64)(unsafe.Pointer(&buf[0]))
		return
	}

	bbuf := bytes.NewBuffer([]byte(val))
	return binary.Read(bbuf, binary.LittleEndian, data)
}
