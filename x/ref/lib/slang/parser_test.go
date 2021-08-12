// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slang

import (
	"go/scanner"
	"path/filepath"
	"testing"
)

func TestSimpleParse(t *testing.T) {
	fp := filepath.Join("testdata", "testparse.slang")
	stmts, err := parseFile(fp)
	if err != nil {
		t.Errorf("failed to parse: %v: %v", fp, err)
	}
	if got, want := stmts[0].String(), `fn1()`; got != want {
		t.Errorf("%s: got %v, want %v", fp, got, want)
	}
	if got, want := stmts.String(), `fn1()
one := fn2()
one, two := fn3(a)
one, two := fn3(a, b)
one, two = fn3(a, b)
printf("%v", v...)
one := fn4(`+"`"+`a multiline string
another line
`+"`)\n"; got != want {
		t.Errorf("%s: got %v, want %v", fp, got, want)
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct {
		text    string
		errmsg  string
		allerrs string
	}{
		{"+", "1:1: invalid token '+'", ""},
		{"break", "1:1: reserved keyword 'break'", ""},
		{"break + ? * + /", "1:1: reserved keyword 'break' (and 5 more errors)", `1:1: reserved keyword 'break'
1:7: invalid token '+'
1:9: illegal character U+003F '?'
1:9: invalid token 'ILLEGAL'
1:11: invalid token '*'
1:13: invalid token '+'
`},
		{",", "1:1: expected variable name, got ','", ""},
		{":=", "1:1: expected variable name, got ':='", ""},
		{",a := f()", "1:1: expected variable name, got ','", ""},
		{"a, := f()", "1:4: expected variable name, got ':=' (and 3 more errors)", `1:4: expected variable name, got ':='
1:8: expected ',' or ':=', got '('
1:9: expected ',' or ':=', got ')'
1:10: expected ',' or ':=', got ';'
`},
		{"a,,b := f()", "1:3: expected variable name, got ','", ""},
		{"f(,)", "1:3: expected variable or literal, got ',' (and 2 more errors)", `1:3: expected variable or literal, got ','
1:4: expected variable name, got ')'
1:5: expected variable name, got ';'
`},
		{"f(a,)", "1:5: expected variable or literal, got ')' (and 1 more errors)", `1:5: expected variable or literal, got ')'
1:6: expected variable name, got ';'
`},
		{"f(,a)", "1:3: expected variable or literal, got ',' (and 2 more errors)", `1:3: expected variable or literal, got ','
1:5: expected ',' or ':=', got ')'
1:6: expected ',' or ':=', got ';'
`},
		{"f(a,,b)", "1:5: expected variable or literal, got ',' (and 2 more errors)", `1:5: expected variable or literal, got ','
1:7: expected ',' or ':=', got ')'
1:8: expected ',' or ':=', got ';'
`},
		{"a := f)", "1:7: expected '(', got ')' (and 1 more errors)", `1:7: expected '(', got ')'
1:8: expected '(', got ';'
`},
		{"a := (", "1:6: expected function name, got '('", ""},
		{"f)", "1:2: expected ',' or ':=', got ')' (and 1 more errors)", `1:2: expected ',' or ':=', got ')'
1:3: expected ',' or ':=', got ';'
`},
		{"f(a(", "1:4: expected ',' or ')', got '('", ""},
		{"f(a))", "1:5: expected '; or '\\n', got ')'", ""},

		{"x,,y := f(,,)\n,,z:=foo(,,))\nfoo))", "1:3: expected variable name, got ',' (and 5 more errors)", `1:3: expected variable name, got ','
1:11: expected variable or literal, got ','
1:12: expected variable name, got ','
1:13: expected variable name, got ')'
1:14: expected variable name, got ';'
2:1: expected variable name, got ','
`},
		{"f(...)", "1:3: expected variable or literal, got '...' (and 2 more errors)", `1:3: expected variable or literal, got '...'
1:6: expected variable name, got ')'
1:7: expected variable name, got ';'
`},
		{"f(a...,)", "1:7: expected ')' got ',' (and 1 more errors)", `1:7: expected ')' got ','
1:8: expected '; or '\n', got ')'
`},
		{"f(a...,b)", "1:7: expected ')' got ',' (and 2 more errors)", `1:7: expected ')' got ','
1:8: expected '; or '\n', got 'IDENT'
1:9: expected '; or '\n', got ')'
`},
	} {

		_, err := parseBytes([]byte(tc.text))
		if err == nil {
			t.Errorf("expected an error for %v:", tc.text)
			continue
		}
		if got, want := err.Error(), tc.errmsg; got != want {
			t.Errorf("%s: got %v, want %v", tc.text, got, want)
		}
		errs := err.(scanner.ErrorList)
		if len(errs) > 1 {
			allerrs := ""
			for _, err := range errs {
				allerrs += err.Error()
				allerrs += "\n"
			}
			if got, want := allerrs, tc.allerrs; got != want {
				t.Errorf("%s: got %v, want %v", tc.text, got, want)
			}
		}
	}
}
