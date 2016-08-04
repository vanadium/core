// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lockutil

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

func getSystemID() (string, error) {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		b, err := ioutil.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(b)), nil
		}
	}
	fi, err := os.Stat("/proc/1")
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(fi.ModTime().UnixNano(), 10), nil
}
