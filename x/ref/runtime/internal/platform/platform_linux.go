// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package platform

import "syscall"

// GetPlatform returns the description of the Platform this process is running on.
// A default value for Platform is provided even if an error is
// returned; nil is never returned for the first return result.
func GetPlatform() (*Platform, error) {
	var uts syscall.Utsname
	if err := syscall.Uname(&uts); err != nil {
		return &Platform{}, err
	}
	d := &Platform{
		Vendor:  "google",
		Model:   "generic",
		System:  utsStr(uts.Sysname[:]),
		Version: utsStr(uts.Version[:]),
		Release: utsStr(uts.Release[:]),
		Machine: utsStr(uts.Machine[:]),
		Node:    utsStr(uts.Nodename[:]),
	}
	return d, nil
}
