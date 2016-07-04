// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

var (
	osVersionFiles = map[string]string{ // file name to name of field within that file
		"/etc/lsb-release": "DISTRIB_DESCRIPTION",
		"/etc/os-release":  "PRETTY_NAME",
	}
)

func detectOSVersion() string {
	for file, key := range osVersionFiles {
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, key) && len(line) > len(key)+1 {
				f.Close()
				line = line[len(key)+1:]
				if tmp, err := strconv.Unquote(line); err == nil {
					return tmp
				}
				return line
			}
		}
		f.Close()
	}
	return ""
}

func detectCPUDescription() string {
	proc, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer proc.Close()
	scanner := bufio.NewScanner(proc)
	for scanner.Scan() {
		// looking for a line like:
		// model name      : Intel(R) Xeon(R) CPU           X5679  @ 3.20GHz
		line := strings.TrimSpace(scanner.Text())
		if tmp, err := strconv.Unquote(line); err == nil {
			line = tmp
		}
		if fields := strings.Fields(line); len(fields) > 3 && fields[0] == "model" && fields[1] == "name" && fields[2] == ":" {
			return strings.Join(fields[3:], " ")
		}
	}
	return ""
}

func detectCPUClockSpeedMHz() uint32 {
	proc, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	defer proc.Close()
	scanner := bufio.NewScanner(proc)
	for scanner.Scan() {
		// looking for a line like:
		// cpu MHz         : 3200.172
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) > 3 && fields[0] == "cpu" && fields[1] == "MHz" && fields[2] == ":" {
			if freq, err := strconv.ParseFloat(fields[3], 32); err == nil {
				return uint32(freq)
			}
		}
	}
	return 0
}
