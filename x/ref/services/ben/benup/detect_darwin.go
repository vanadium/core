// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"os/exec"
	"strings"
	"syscall"
)

func detectOSVersion() string {
	out, _ := exec.Command("sw_vers").Output()
	reader := bufio.NewReader(bytes.NewBuffer(out))
	// Each line of output is of the form: "Key: Value", so concatenate all the values
	var lines []string
	for {
		if _, err := reader.ReadString(':'); err != nil {
			break
		}
		piece, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		lines = append(lines, strings.TrimSpace(piece))
	}
	return strings.Join(lines, " ")
}

func detectCPUDescription() string {
	ret, _ := syscall.Sysctl("machdep.cpu.brand_string")
	return ret
}

func detectCPUClockSpeedMHz() uint32 {
	bo, err := syscall.SysctlUint32("hw.byteorder")
	if err != nil {
		return 0
	}
	str, err := syscall.Sysctl("hw.cpufrequency_max")
	if err != nil {
		return 0
	}
	var bytes8 [8]byte
	bytes := []byte(str)
	var hz uint64
	switch bo {
	case 1234:
		copy(bytes8[:], bytes)
		hz = binary.LittleEndian.Uint64(bytes8[:])
	case 4321:
		copy(bytes8[8-len(bytes):], bytes)
		hz = binary.BigEndian.Uint64(bytes8[:])
	}
	return uint32(hz / 1000000)
}
