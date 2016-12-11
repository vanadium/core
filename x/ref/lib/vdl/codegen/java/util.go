// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"unicode"
	"unicode/utf8"
)

// accessModifierForName returns the Java access modifier given the name.
// It follows VDL naming conventions, indicating that an uppercase name
// denotes a public type and a lowercase name a package-protected type.
func accessModifierForName(name string) string {
	r, _ := utf8.DecodeRuneInString(name)
	if unicode.IsUpper(r) {
		return "public"
	}
	return ""
}
