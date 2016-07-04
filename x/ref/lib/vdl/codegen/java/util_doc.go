// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bufio"
	"strings"
)

// javaDoc transforms the provided VDL doc and a doc suffix into the JavaDoc format.
func javaDoc(doc, suffix string) string {
	if doc == "" && suffix == "" {
		return ""
	}
	if doc == "" {
		doc = suffix
	} else if suffix != "" {
		doc = doc + "\n" + suffix
	}
	ret := "/**\n"
	reader := bufio.NewReader(strings.NewReader(doc))
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//") {
			ret += " *"
			if len(line[2:]) == 0 { // empty line
				ret += "<p>"
			} else {
				ret += line[2:]
			}
			ret += "\n"
		}
		if err != nil {
			break
		}
	}
	ret += "*/"
	return ret
}
