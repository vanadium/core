// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package glob

import (
	"testing"
)

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStripFixedElements(t *testing.T) {
	tests := []struct {
		pattern string
		fixed   []string
		rest    string
	}{
		{"*", nil, "*"},
		{"a/b/c/*", []string{"a", "b", "c"}, "*"},
		{"a/b/*/...", []string{"a", "b"}, "*/..."},
		{"a/b/c/...", []string{"a", "b", "c"}, "..."},
		{"a/the\\?rain.in\\*spain", []string{"a", "the?rain.in*spain"}, ""},
	}
	for _, test := range tests {
		g, err := Parse(test.pattern)
		if err != nil {
			t.Fatalf("parsing %q: %q", test.pattern, err.Error())
		}
		if f, ng := g.SplitFixedElements(); !same(f, test.fixed) || test.rest != ng.String() {
			t.Fatalf("SplitFixedElements(%q) got %q,%q, expected %q,%q", test.pattern, f, ng.String(), test.fixed, test.rest)
		}
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		matched bool
	}{
		{"...", "", true},
		{"***", "", true},
		{"...", "a", true},
		{"***", "a", true},
		{"a", "", false},
		{"a", "a", true},
		{"a", "b", false},
		{"a*", "a", true},
		{"a*", "b", false},
		{"a*b", "ab", true},
		{"a*b", "afoob", true},
		{"a\\*", "a", false},
		{"a\\*", "a*", true},
		{"\\\\", "\\", true},
		{"?", "?", true},
		{"?", "a", true},
		{"?", "", false},
		{"*?", "", false},
		{"*?", "a", true},
		{"*?", "ab", true},
		{"*?", "abv", true},
		{"[abc]", "a", true},
		{"[abc]", "b", true},
		{"[abc]", "c", true},
		{"[abc]", "d", false},
		{"[a-c]", "a", true},
		{"[a-c]", "b", true},
		{"[a-c]", "c", true},
		{"[a-c]", "d", false},
		{"\\[abc]", "a", false},
		{"\\[abc]", "[abc]", true},
		{"a/*", "a", true},
		{"a/*", "b", false},
		{"a/...", "a", true},
		{"a/...", "b", false},
	}
	for i, test := range tests {
		g, err := Parse(test.pattern)
		if err != nil {
			t.Errorf("unexpected parsing error for %q (#%d): %v", test.pattern, i, err)
			continue
		}
		if matched := g.Head().Match(test.name); matched != test.matched {
			t.Errorf("unexpected result for %q.Match(%q) (#%d). Got %v, expected %v", test.pattern, test.name, i, matched, test.matched)
		}
	}
}

func TestFixedPrefix(t *testing.T) {
	tests := []struct {
		pattern string
		prefix  string
		full    bool
	}{
		{"", "", true},
		{"...", "", false},
		{"***", "", false},
		{"...", "", false},
		{"***", "", false},
		{"a", "a", true},
		{"*a", "", false},
		{"a*", "a", false},
		{"a*b", "a", false},
		{"a\\*", "a*", true},
		{"\\\\", "\\", true},
		{"?", "", false},
		{"\\?", "?", true},
		{"*?", "", false},
		{"[abc]", "", false},
		{"\\[abc]", "[abc]", true},
		{"\\[abc]*", "[abc]", false},
	}
	for i, test := range tests {
		g, err := Parse(test.pattern)
		if err != nil {
			t.Errorf("unexpected parsing error for %q (#%d): %v", test.pattern, i, err)
			continue
		}
		if prefix, full := g.Head().FixedPrefix(); prefix != test.prefix || full != test.full {
			t.Errorf("unexpected result for %q.FixedPrefix() (#%d). Got (%q,%v), expected (%q,%v)", test.pattern, i, prefix, full, test.prefix, test.full)
		}
	}
}

func TestBadPattern(t *testing.T) {
	tests := []string{"[", "[foo", "[^foo", "\\", "a\\", "abc[foo", "a//b"}
	for _, test := range tests {
		if _, err := Parse(test); err == nil {
			t.Errorf("Unexpected success for %q", test)
		}
	}
}
