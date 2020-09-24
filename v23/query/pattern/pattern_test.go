// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pattern

import (
	"errors"
	"testing"
)

type patternTest struct {
	pattern         string
	escChar         rune
	wantRegex       string
	wantFixedPrefix string
	wantNoWildcards bool
	matches         []string
	nonMatches      []string
}

func TestPatternParse(t *testing.T) {
	tests := []patternTest{
		{
			"abc", '\\',
			"^abc$",
			"abc", true,
			[]string{"abc"},
			[]string{"a", "abcd", "xabc"},
		},
		{
			"abc%", '\\',
			"^abc.*?$",
			"abc", false,
			[]string{"abc", "abcd", "abcabc"},
			[]string{"xabcd"},
		},
		{
			"abc_", '\\',
			"^abc.$",
			"abc", false,
			[]string{"abcd", "abc1"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"*%*_%", '*',
			"^%_.*?$",
			"%_", false,
			[]string{"%_", "%_abc"},
			[]string{"%a", "abc%_abc"},
		},
		{
			"abc^_", '^',
			"^abc_$",
			"abc_", true,
			[]string{"abc_"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"abc^%", '^',
			"^abc%$",
			"abc%", true,
			[]string{"abc%"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"abc^^%", '^',
			"^abc\\^.*?$",
			"abc^", false,
			[]string{"abc^", "abc^de", "abc^%"},
			[]string{"abc", "abcd", "xabc^d"},
		},
		{
			"abc_efg", '\\',
			"^abc.efg$",
			"abc", false,
			[]string{"abcdefg"},
			[]string{"abc", "xabcd", "abcde", "abcdefgh"},
		},
		{
			"abc%def", '\\',
			"^abc.*?def$",
			"abc", false,
			[]string{"abcdefdef", "abcdef", "abcdefghidef"},
			[]string{"abcdefg", "abcdefde"},
		},
		{
			"[0-9]*abc%def", '\\',
			"^\\[0-9\\]\\*abc.*?def$",
			"[0-9]*abc", false,
			[]string{"[0-9]*abcdefdef", "[0-9]*abcdef", "[0-9]*abcdefghidef"},
			[]string{"0abcdefg", "9abcdefde", "[0-9]abcdefg", "[0-9]abcdefg", "[0-9]abcdefg"},
		},
		{
			"abc\x00%def\x00", '\x00', // nul character doesn't escape
			"^abc\x00.*?def\x00$",
			"abc\x00", false,
			[]string{"abc\x00defdef\x00", "abc\x00def\x00", "abc\x00defghidef\x00"},
			[]string{"abcdef\x00", "abc%def", "abc\x00defg", "abc\x00def\x00de", "abc\x00defdef"},
		},
	}

	for _, test := range tests {
		p, err := ParseWithEscapeChar(test.pattern, test.escChar)
		if err != nil {
			t.Fatalf("failed to parse pattern %q: %v", test.pattern, err)
		}
		if got, want := p.regex.String(), test.wantRegex; got != want {
			t.Errorf("pattern %q: expected regex %q, got %q", test.pattern, want, got)
		}
		if got, want := p.fixedPrefix, test.wantFixedPrefix; got != want {
			t.Errorf("pattern %q: expected fixedPrefix %q, got %q", test.pattern, want, got)
		}
		if got, want := p.noWildcards, test.wantNoWildcards; got != want {
			t.Errorf("pattern %q: expected noWildcards %v, got %v", test.pattern, want, got)
		}
		// Make sure all expected matches actually match.
		for _, m := range test.matches {
			if !p.MatchString(m) {
				t.Errorf("pattern %q: expected %q to match", test.pattern, m)
			}
		}
		// Make sure all expected nonMatches actually don't match.
		for _, n := range test.nonMatches {
			if p.MatchString(n) {
				t.Errorf("pattern %q: expected %q to not match", test.pattern, n)
			}
		}
	}
}

func TestPatternParseError(t *testing.T) {
	escChar := '#'
	errorPatterns := []string{
		"#",
		"abc#",
		"a#bc",
		"a%bc##de#_f#gh",
		"a%bc##de#_f#",
		"###",
		"###a_",
		"a#bc%de",
	}

	for _, p := range errorPatterns {
		if _, err := ParseWithEscapeChar(p, escChar); !errors.Is(err, ErrInvalidEscape) {
			t.Errorf("pattern %q: expected ErrInvalidEscape, got %v", p, err)
		}
	}
}

type escapeTest struct {
	literal     string
	escChar     rune
	wantPattern string
}

func TestPatternEscape(t *testing.T) {
	tests := []escapeTest{
		{
			"abc", '#',
			"abc",
		},
		{
			"abcabc", 'b',
			"abbcabbc",
		},
		{
			"ab%c", '#',
			"ab#%c",
		},
		{
			"_abc%&", '&',
			"&_abc&%&&",
		},
		{
			"___", '^',
			"^_^_^_",
		},
		{
			"abc%", '\\',
			"abc\\%",
		},
		{
			"_同步数据库_", '钒',
			"钒_同步数据库钒_",
		},
	}

	for _, test := range tests {
		ep := EscapeWithEscapeChar(test.literal, test.escChar)
		if got, want := ep, test.wantPattern; got != want {
			t.Errorf("literal %q: expected escape %q, got %q", test.literal, want, got)
		}
		if p, err := ParseWithEscapeChar(ep, test.escChar); err != nil {
			t.Fatalf("failed to parse escaped pattern %q: %v", ep, err)
		} else if !p.noWildcards {
			t.Errorf("literal %q: escaped pattern contains unescaped wildcards", test.literal)
		}
	}
}
