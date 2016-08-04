// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"

	"v.io/x/ref/lib/vdl/compile"
)

// javaFileNames constructs a comma separated string with the short (basename) of the input files
func javaFileNames(files []*compile.File) string {
	var buf bytes.Buffer
	for i, file := range files {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(file.BaseName)
	}
	return buf.String()
}
