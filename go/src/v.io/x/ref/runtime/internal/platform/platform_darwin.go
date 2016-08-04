// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package platform

// #include <sys/utsname.h>
// #include <errno.h>
import "C"

import "fmt"

// GetPlatform returns the description of the Platform this process is running on.
// A default value for Platform is provided even if an error is
// returned; nil is never returned for the first return result.
func GetPlatform() (*Platform, error) {
	var t C.struct_utsname
	if r, err := C.uname(&t); r != 0 {
		return &Platform{}, fmt.Errorf("uname failed: errno %d", err)
	}
	d := &Platform{
		Vendor:  "google",
		Model:   "generic",
		System:  C.GoString(&t.sysname[0]),
		Version: C.GoString(&t.version[0]),
		Release: C.GoString(&t.release[0]),
		Machine: C.GoString(&t.machine[0]),
		Node:    C.GoString(&t.nodename[0]),
	}
	return d, nil
}
