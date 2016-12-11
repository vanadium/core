// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lockutil

import (
	"fmt"
	"os/exec"
	"strings"
)

// We use the mac serial number to identify the system
// (https://developer.apple.com/library/mac/technotes/tn1103/_index.html).  The
// most convenient and fastest way we found to obtain this is by scraping the
// output of:
//   ioreg -c IOPlatformExpertDevice -d 2
const serialNumberLabel = "\"IOPlatformSerialNumber\""

func getSystemID() (string, error) {
	var output string
	if out, err := exec.Command("ioreg", "-c", "IOPlatformExpertDevice", "-d", "2").Output(); err != nil {
		return "", err
	} else {
		output = string(out)
	}
	// We're looking for a line like this:
	//     "IOPlatformSerialNumber" = "F2GMN0QMF7VD"
	if i := strings.Index(output, serialNumberLabel); i >= 0 {
		output = strings.TrimPrefix(output[i:], serialNumberLabel)
		if i := strings.Index(output, "\n"); i >= 0 {
			if id := strings.Trim(output[:i], "\" ="); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("failed to determine serial number")
}
