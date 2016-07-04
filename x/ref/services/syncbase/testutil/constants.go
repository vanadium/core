// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"strings"
)

var invalidBlessingPatterns = []string{
	":",
	"a:",
	":b",
	"a::b",
	"a/b",
	"a/",
	"$",
}

var validBlessingPatterns = []string{
	"a",
	"a:b",
	"a:%",
	"a:%A",
	"v.io",
	"v.io:foo",
	"v.io:foobar",
	"v.io:foo:bar",
	"v.io:a:admin@myapp.com",
	"v.io:o:app:user",
	"\x00",
	"\xfb",
	"a\x00",
	"a\xfb",
	"안녕하세요",
	// 16 4-byte runes => 64 bytes
	strings.Repeat("𠜎", 16),
}

var invalidIdentifiers = []string{
	"/",
	"a/b",
	":",
	"a:b",
	"*",
	"\x00",
	"\x01",
	"\xfa",
	"\xfb",
	"@@",
	"dev.v.io/a/admin@myapp.com",
	"dev.v.io:a:admin@myapp.com",
	"안녕하세요",
	// 16 4-byte runes => 64 bytes
	strings.Repeat("𠜎", 16),
}

var validIdentifiers = []string{
	"a",
	"B",
	"a_",
	"a__",
	"a0_",
	"a_b",
	"a_0",
	"foo",
	"foobar",
	"foo_bar",
	"BARBAZ",
	// 64 bytes
	"abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcd",
}

var universallyInvalidNames = []string{
	"",
	"\xfc",
	"\xfd",
	"\xfe",
	"\xff",
	"a\xfcb",
	"a\xfdb",
	"a\xfeb",
	"a\xffb",
}

var longNames = []string{
	// 65 bytes
	"abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcde",
	// 16 4-byte runes + 1 more byte => 65 bytes
	strings.Repeat("𠜎", 16) + "a",
	// 256 4-byte runes => 1024 bytes
	strings.Repeat("𠜎", 256),
}

var veryLongNames = []string{
	// 256 4-byte runes + 1 more byte => 1025 bytes
	strings.Repeat("𠜎", 256) + "a",
	strings.Repeat("foobar", 1337),
}

func concat(slices ...[]string) []string {
	var res []string
	for _, slice := range slices {
		res = append(res, slice...)
	}
	return res
}

var (
	OkAppUserBlessings    []string = concat(validBlessingPatterns, longNames)
	NotOkAppUserBlessings []string = concat(universallyInvalidNames, invalidBlessingPatterns, veryLongNames)

	OkDbCxNames    []string = validIdentifiers
	NotOkDbCxNames []string = concat(universallyInvalidNames, invalidIdentifiers, longNames, veryLongNames)
)

var (
	OkRowKeys    []string = concat(validIdentifiers, invalidIdentifiers, longNames, veryLongNames)
	NotOkRowKeys []string = universallyInvalidNames
)
