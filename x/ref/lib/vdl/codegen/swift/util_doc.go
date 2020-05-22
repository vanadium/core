// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"bufio"
	"strings"
)

// swiftDoc transforms the provided VDL doc and a doc suffix into the SwiftDoc format.
func swiftDoc(doc, suffix string) string {
	if doc == "" && suffix == "" {
		return ""
	}
	if doc == "" {
		doc = suffix
	} else if suffix != "" {
		doc = doc + "\n" + suffix
	}
	ret := ""
	reader := bufio.NewReader(strings.NewReader(doc))
	hitContent := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// A purely empty comment gets ignored
			if !hitContent {
				ret = ""
			}
			return ret
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//") {
			// Three consecutive slashes is the format for Swiftdoc. Add the additional beyond what the line starts with.
			ret += "/"
			ret += line
			ret += "\n"
			if !hitContent && line != "//" {
				hitContent = true
			}
		}
	}
}
