// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidConstIdent(t *testing.T) {
	tests := []struct {
		name     string
		exported bool
		errstr   string
	}{
		{"", false, "allowed regexp"},
		{"0FirstLetterDigit", false, "allowed regexp"},
		{"_FirstLetterPunct", false, "allowed regexp"},
		{" FirstLetterSpace", false, "allowed regexp"},
		{"x.InvalidPunct", false, "allowed regexp"},
		{"x InvalidSpace", false, "allowed regexp"},
		{"x\nNonAlphaNum", false, "allowed regexp"},
		{"ABC", true, ""},
		{"xxABC", false, ""},
		{"xxABCyy", false, ""},
		{"ID", true, ""},
		{"ID_", true, ""},
		{"ID2", true, ""},
		{"xxID", false, ""},
		{"xxIDyy", false, ""},
		{"A_M", true, ""},
		{"B2B", true, ""},
		{"X", true, ""},
		{"Xyz", true, ""},
		{"Xyz123", true, ""},
		{"Xyz_123", true, ""},
		{"x", false, ""},
		{"xyz", false, ""},
		{"xyz123", false, ""},
		{"xyz_123", false, ""},
	}
	for _, test := range tests {
		exported, err := validConstIdent(test.name, reservedNormal)
		errstr := fmt.Sprint(err)
		if test.errstr != "" && !strings.Contains(errstr, test.errstr) {
			t.Errorf(`validConstIdent(%s) got error %q, want substr %q`, test.name, errstr, test.errstr)
		}
		if test.errstr == "" && err != nil {
			t.Errorf(`validConstIdent(%s) got error %q, want nil`, test.name, errstr)
		}
		if got, want := exported, test.exported; got != want {
			t.Errorf(`validConstIdent(%s) got exported %v, want %v`, test.name, got, want)
		}
	}
}

func TestValidIdent(t *testing.T) {
	tests := []struct {
		name     string
		exported bool
		errstr   string
	}{
		{"", false, "allowed regexp"},
		{"0FirstLetterDigit", false, "allowed regexp"},
		{"_FirstLetterPunct", false, "allowed regexp"},
		{" FirstLetterSpace", false, "allowed regexp"},
		{"x.InvalidPunct", false, "allowed regexp"},
		{"x InvalidSpace", false, "allowed regexp"},
		{"x\nNonAlphaNum", false, "allowed regexp"},
		{"ABC", false, "acronyms must use CamelCase"},
		{"xxABC", false, "acronyms must use CamelCase"},
		{"xxABCyy", false, "acronyms must use CamelCase"},
		{"ID", false, "acronyms must use CamelCase"},
		{"ID_", false, "acronyms must use CamelCase"},
		{"ID2", false, "acronyms must use CamelCase"},
		{"xxID", false, "acronyms must use CamelCase"},
		{"xxIDyy", false, ""},
		{"A_M", true, ""},
		{"B2B", true, ""},
		{"X", true, ""},
		{"Xyz", true, ""},
		{"Xyz123", true, ""},
		{"Xyz_123", true, ""},
		{"x", false, ""},
		{"xyz", false, ""},
		{"xyz123", false, ""},
		{"xyz_123", false, ""},
	}
	for _, test := range tests {
		exported, err := validIdent(test.name, reservedNormal)
		errstr := fmt.Sprint(err)
		if test.errstr != "" && !strings.Contains(errstr, test.errstr) {
			t.Errorf(`validIdent(%s) got error %q, want substr %q`, test.name, errstr, test.errstr)
		}
		if test.errstr == "" && err != nil {
			t.Errorf(`validIdent(%s) got error %q, want nil`, test.name, errstr)
		}
		if got, want := exported, test.exported; got != want {
			t.Errorf(`validIdent(%s) got exported %v, want %v`, test.name, got, want)
		}
	}
}

func TestValidExportedIdent(t *testing.T) {
	tests := []struct {
		ident  string
		errstr string
	}{
		{"", "allowed regexp"},
		{"xFirstLetterLower", "must be exported"},
		{"0FirstLetterDigit", "allowed regexp"},
		{"_FirstLetterPunct", "allowed regexp"},
		{" FirstLetterSpace", "allowed regexp"},
		{"X.InvalidPunct", "allowed regexp"},
		{"X InvalidSpace", "allowed regexp"},
		{"X\nNonAlphaNum", "allowed regexp"},
		{"ABC", "acronyms must use CamelCase"},
		{"XxABC", "acronyms must use CamelCase"},
		{"XxABCyy", "acronyms must use CamelCase"},
		{"ID", "acronyms must use CamelCase"},
		{"ID_", "acronyms must use CamelCase"},
		{"ID2", "acronyms must use CamelCase"},
		{"XxID", "acronyms must use CamelCase"},
		{"XxIDyy", ""},
		{"A_M", ""},
		{"B2B", ""},
		{"X", ""},
		{"Xyz", ""},
		{"Xyz123", ""},
		{"Xyz_123", ""},
	}
	for _, test := range tests {
		err := validExportedIdent(test.ident, reservedNormal)
		errstr := fmt.Sprint(err)
		if test.errstr != "" && !strings.Contains(errstr, test.errstr) {
			t.Errorf(`validExportedIdent(%s) got error %q, want substr %q`, test.ident, errstr, test.errstr)
		}
		if test.errstr == "" && err != nil {
			t.Errorf(`validExportedIdent(%s) got error %q, want nil`, test.ident, errstr)
		}
	}
}
