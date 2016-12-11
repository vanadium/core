// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlutil

import (
	"testing"
)

func TestFirstRuneToLower(t *testing.T) {
	tests := []struct {
		arg, want string
	}{
		{"foo", "foo"},
		{"Foo", "foo"},
		{"FOO", "fOO"},
		{"foobar", "foobar"},
		{"fooBar", "fooBar"},
		{"FooBar", "fooBar"},
		{"FOOBAR", "fOOBAR"},
	}
	for _, test := range tests {
		if got, want := FirstRuneToLower(test.arg), test.want; got != want {
			t.Errorf("FirstRuneToLower(%s) got %s, want %s", test.arg, got, want)
		}
	}
}

func TestFirstRuneToUpper(t *testing.T) {
	tests := []struct {
		arg, want string
	}{
		{"foo", "Foo"},
		{"Foo", "Foo"},
		{"FOO", "FOO"},
		{"foobar", "Foobar"},
		{"fooBar", "FooBar"},
		{"FooBar", "FooBar"},
		{"FOOBAR", "FOOBAR"},
	}
	for _, test := range tests {
		if got, want := FirstRuneToUpper(test.arg), test.want; got != want {
			t.Errorf("FirstRuneToUpper(%s) got %s, want %s", test.arg, got, want)
		}
	}
}

func TestConstCase(t *testing.T) {
	testcases := []struct {
		name, want string
	}{
		{"TestFunction", "TEST_FUNCTION"},
		{"BIGNumber", "BIG_NUMBER"},
		{"SHA256Hash", "SHA_256_HASH"},
		{"Sha256Hash", "SHA_256_HASH"},
		{"Sha256hash", "SHA_256_HASH"},
		{"THISIsAHugeVarname", "THIS_IS_A_HUGE_VARNAME"},
		{"Sha256MD5Function", "SHA_256_MD_5_FUNCTION"},
		{"IfIHadADollar4EachTest", "IF_I_HAD_A_DOLLAR_4_EACH_TEST"},
	}

	for _, testcase := range testcases {
		if want, got := testcase.want, ToConstCase(testcase.name); want != got {
			t.Errorf("toConstCase(%q) error, want %q, got %q", testcase.name, want, got)
		}
	}
}
