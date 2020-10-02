// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"strings"
)

// javaPath converts the provided Go path into the Java path.  It replaces all "/"
// with "." in the path.
func javaPath(goPath string) string {
	return strings.ReplaceAll(goPath, "/", ".")
}
