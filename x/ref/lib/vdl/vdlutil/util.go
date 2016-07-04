// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vdlutil implements utilities used by many VDL components.  It should
// have a small set of dependencies.
package vdlutil

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// FirstRuneToLower returns s with its first rune in lowercase.
func FirstRuneToLower(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

// FirstRuneToUpper returns s with its first rune in uppercase.
func FirstRuneToUpper(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// FirstRuneToExportCase returns s with its first rune in uppercase if export is
// true, otherwise in lowercase.
func FirstRuneToExportCase(s string, export bool) string {
	if export {
		return FirstRuneToUpper(s)
	}
	return FirstRuneToLower(s)
}

// toConstCase converts ThisString to THIS_STRING. For adding '_', we follow the
// following algorithm.  For any sequence of three characters, c[n-1], c[n],
// c[n+1], we add an underscore before c[n] if:
//     1) c[n-1] is a digit and c[n] is a letter, or
//     2) c[n-1] is a letter and c[n] is a digit, or
//     3) c[n-1] is lowercase, and c[n] is uppercase, or
//     4) c[n-1] is uppercase, c[n] is uppercase, and c[n+1] is lowercase.
func ToConstCase(s string) string {
	var buf bytes.Buffer
	var size int
	var prev, cur, next rune
	next, size = utf8.DecodeRuneInString(s)
	for next != utf8.RuneError {
		s = s[size:]
		prev, cur = cur, next
		next, size = utf8.DecodeRuneInString(s)
		// We avoid checking boundary conditions because, for a rune r that is zero
		// or utf8.RuneError: unicode.Is{Letter,Digit,Lower,Upper}(r) == false
		if unicode.IsDigit(prev) && unicode.IsLetter(cur) || // Rule (1)
			unicode.IsLetter(prev) && unicode.IsDigit(cur) || // Rule (2)
			unicode.IsLower(prev) && unicode.IsUpper(cur) || // Rule (3)
			unicode.IsUpper(prev) && unicode.IsUpper(cur) && unicode.IsLower(next) { // Rule (4)
			buf.WriteRune('_')
		}
		buf.WriteRune(unicode.ToUpper(cur))
	}
	return buf.String()
}
