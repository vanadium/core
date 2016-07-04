// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import (
	"testing"
)

func TestSplitAddressName(t *testing.T) {
	cases := []struct {
		input, address, name string
	}{
		{"", "", ""},
		{"/", "", ""},
		{"//", "", ""},
		{"//abc@@host/foo", "abc@@host", "foo"},
		{"a", "", "a"},
		{"/a", "a", ""},
		{"/a/", "a", ""},
		{"a/b", "", "a/b"},
		{"/a/b", "a", "b"},
		{"abc@@/foo", "", "abc@@/foo"},
		{"/abc@@host/foo", "abc@@host", "foo"},
		{"/abc/foo", "abc", "foo"},
		{"/abc/foo//x", "abc", "foo/x"},
		{"/abc:20/foo", "abc:20", "foo"},
		{"/abc//foo/bar", "abc", "foo/bar"},
		{"/0abc:20/foo", "0abc:20", "foo"},
		{"/abc1.2:20/foo", "abc1.2:20", "foo"},
		{"/abc:xx/foo", "abc:xx", "foo"},
		{"/-abc/foo", "-abc", "foo"},
		{"/a.-abc/foo", "a.-abc", "foo"},
		{"/[01:02::]:444", "[01:02::]:444", ""},
		{"/[01:02::]:444/foo", "[01:02::]:444", "foo"},
		{"/12.3.4.5:444", "12.3.4.5:444", ""},
		{"/12.3.4.5:444/foo", "12.3.4.5:444", "foo"},
		{"/12.3.4.5", "12.3.4.5", ""},
		{"/12.3.4.5/foo", "12.3.4.5", "foo"},
		{"/12.3.4.5//foo", "12.3.4.5", "foo"},
		{"/12.3.4.5/foo//bar", "12.3.4.5", "foo/bar"},
		{"/user@domain.com@host:1234/foo/bar", "user@domain.com@host:1234", "foo/bar"},
		{"/(dev.v.io/services/mounttabled)@host:1234/foo/bar", "(dev.v.io/services/mounttabled)@host:1234", "foo/bar"},
		{"/(dev.v.io/services/mounttabled)@host:1234/", "(dev.v.io/services/mounttabled)@host:1234", ""},
		{"/(dev.v.io/services/mounttabled)@host:1234", "(dev.v.io/services/mounttabled)@host:1234", ""},
		{"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io/", "@4@tcp@127.0.0.1:22@@@@s@dev.v.io", ""}, // malformed endpoint, doesn't end in a @@
		{"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io", "@4@tcp@127.0.0.1:22@@@@s@dev.v.io", ""},  // malformed endpoint, doesn't end in a @@
		{"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io/services/mounttabled@@/foo/bar", "@4@tcp@127.0.0.1:22@@@@s@dev.v.io/services/mounttabled@@", "foo/bar"},
		{"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io/services/mounttabled,staging.v.io/services/nsroot@@/foo/bar", "@4@tcp@127.0.0.1:22@@@@s@dev.v.io/services/mounttabled,staging.v.io/services/nsroot@@", "foo/bar"},
		{"/@@@127.0.0.1:22@@@@/foo/bar", "@@@127.0.0.1:22@@@@", "foo/bar"},
		{"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io/services/mounttabled,staging.v.io/services/nsroot@@", "@4@tcp@127.0.0.1:22@@@@s@dev.v.io/services/mounttabled,staging.v.io/services/nsroot@@", ""},
	}

	for _, c := range cases {
		addr, name := SplitAddressName(c.input)
		if addr != c.address || name != c.name {
			t.Errorf("SplitAddressName(%q): got %q,%q want %q,%q", c.input, addr, name, c.address, c.name)
		}
	}
}

func BenchmarkSplitAddressName(b *testing.B) {
	tests := []string{
		"/user@domain.com@host:1234/foo/bar",
		"/(dev.v.io/services/mounttabled)@host:1234/foo/bar",
		"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io/services/mounttabled@@/foo/bar",
		"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io/services/mounttabled,staging.v.io/services/nsroot@@/foo/bar",
	}
	for i := 0; i < b.N; i++ {
		SplitAddressName(tests[i%len(tests)])
	}
}

func TestJoinAddressName(t *testing.T) {
	cases := []struct {
		address, name, joined string
	}{
		{"", "", ""},
		{"", "a", "a"},
		{"", "/a", "/a"},
		{"", "a", "a"},
		{"", "///a", "/a"},
		{"/", "", ""},
		{"//", "", ""},
		{"/a", "", "/a"},
		{"//a", "", "/a"},
		{"aaa", "", "/aaa"},
		{"/aaa", "aa", "/aaa/aa"},
		{"ab", "/cd", "/ab/cd"},
		{"/ab", "/cd", "/ab/cd"},
		{"ab", "//cd", "/ab/cd"},
	}
	for _, c := range cases {
		joined := JoinAddressName(c.address, c.name)
		if joined != c.joined {
			t.Errorf("JoinAddressName(%q %q): got %q want %q", c.address, c.name, joined, c.joined)
		}
	}
}

func TestJoin(t *testing.T) {
	cases := []struct {
		elems  []string
		joined string
	}{
		{[]string{}, ""},
		{[]string{""}, ""},
		{[]string{"", ""}, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", ""}, "a"},
		{[]string{"a/"}, "a"},
		{[]string{"a/", ""}, "a"},
		{[]string{"a", "/"}, "a"},
		{[]string{"", "a"}, "a"},
		{[]string{"", "/a"}, "/a"},
		{[]string{"a", "b"}, "a/b"},
		{[]string{"a/", "b/"}, "a/b"},
		{[]string{"a/", "/b"}, "a/b"},
		{[]string{"/a", "b"}, "/a/b"},
		{[]string{"a", "/", "b"}, "a/b"},
		{[]string{"a", "/", "/b"}, "a/b"},
		{[]string{"a/", "/", "/b"}, "a/b"},
		{[]string{"/a/b", "c"}, "/a/b/c"},
		{[]string{"/a", "b", "c"}, "/a/b/c"},
		{[]string{"/a/", "/b/", "/c/"}, "/a/b/c"},
		{[]string{"a", "b", "c"}, "a/b/c"},
		{[]string{"a", "", "c"}, "a/c"},
		{[]string{"a", "", "", "c"}, "a/c"},
		{[]string{"/a/b", "c/d"}, "/a/b/c/d"},
		{[]string{"/a/b", "/c/d"}, "/a/b/c/d"},
		{[]string{"/a/b", "//c/d"}, "/a/b/c/d"},
		{[]string{"/a//", "c"}, "/a/c"},
		{[]string{"/a", "//"}, "/a"},
		{[]string{"", "//a/b"}, "/a/b"},
		{[]string{"a", "b//"}, "a/b"},
		{[]string{"a", "//", "b"}, "a/b"},
		{[]string{"a", "//", "/b"}, "a/b"},
		{[]string{"a", "//", "//b"}, "a/b"},
		{[]string{"a/", "//", "b"}, "a/b"},
		{[]string{"a//", "//", "b"}, "a/b"},
		{[]string{"a//", "//", "//b"}, "a/b"},
		{[]string{"a", "/", "/", "b"}, "a/b"},
		{[]string{"a/", "/", "/", "/b"}, "a/b"},
		{[]string{"a", "//", "//", "b"}, "a/b"},
		{[]string{"a//", "//", "//", "//b"}, "a/b"},
		{[]string{"a//", "//b//", "//c//"}, "a/b/c"},
		{[]string{"a//", "", "//c//"}, "a/c"},
		{[]string{"a///", "////b"}, "a/b"},
		{[]string{"////a", "b"}, "/a/b"},
		{[]string{"a", "b////"}, "a/b"},
		{[]string{"/ep//", ""}, "/ep"},
		{[]string{"/ep//", "a"}, "/ep/a"},
		{[]string{"/ep//", "//a"}, "/ep/a"},
	}
	for _, c := range cases {
		if got, want := Join(c.elems...), c.joined; want != got {
			t.Errorf("Join(%q): got %q want %q", c.elems, got, want)
		}
	}
}

func TestSplitJoin(t *testing.T) {
	cases := []struct {
		name, address, relative string
	}{
		{"/a/b", "a", "b"},
		{"/a//b", "a", "b"},
		{"/a:10//b/c", "a:10", "b/c"},
		{"/a:10/b//c", "a:10", "b/c"},
	}
	for _, c := range cases {
		a, r := SplitAddressName(c.name)
		if got, want := a, c.address; got != want {
			t.Errorf("%q: got %q, want %q", c.name, got, want)
		}
		if got, want := r, c.relative; got != want {
			t.Errorf("%q: got %q, want %q", c.name, got, want)
		}
		j := JoinAddressName(a, r)
		if got, want := j, Clean(c.name); got != want {
			t.Errorf("%q: got %q, want %q", c.name, got, want)
		}
	}
}

func TestTrimSuffix(t *testing.T) {
	cases := []struct {
		name, suffix, prefix string
	}{
		{"", "", ""},
		{"a", "", "a"},
		{"a", "a", ""},
		{"/a", "a", "/a"},
		{"a/b", "b", "a"},
		{"a/b", "/b", "a/b"},
		{"a/b/", "b/", "a"},
		{"/a/b", "b", "/a"},
		{"/a/b/c", "c", "/a/b"},
		{"/a/b/c/d", "c/d", "/a/b"},
		{"/a/b//c/d", "c/d", "/a/b"},
		{"/a/b//c/d", "/c/d", "/a/b/c/d"},
		{"/a/b//c/d", "//c/d", "/a/b/c/d"},
		{"//a/b", "//a/b", ""},
		{"/a/b", "/a/b", ""},
		{"//a", "a", "/a"},
	}
	for _, c := range cases {
		if p := TrimSuffix(c.name, c.suffix); p != c.prefix {
			t.Errorf("TrimSuffix(%q, %q): got %q, want %q", c.name, c.suffix, p, c.prefix)
		}
	}
}

func TestRooted(t *testing.T) {
	ep := "/" + FormatEndpoint("tcp", "h:0")
	// should be rooted.
	cases := []string{
		"/",
		"/a",
		"/a/b",
		ep + "/",
	}
	for _, c := range cases {
		if !Rooted(c) {
			t.Errorf("Rooted(%q) return false, not true", c)
		}

	}
	cases = []string{
		"",
		"a",
		"b//c",
	}
	for _, c := range cases {
		if Rooted(c) {
			t.Errorf("Rooted(%q) return true, not false", c)
		}

	}

}

func TestClean(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"//", "/"},
		{"/", "/"},
		{"/a//b", "/a/b"},
		{"/a//b/", "/a/b"},
		{"a//b", "a/b"},
		{"a//b/", "a/b"},
		{"///a//b/", "/a/b"},
		{"a////////b/", "a/b"},
	}
	for _, c := range cases {
		if want, got := c.want, Clean(c.in); got != want {
			t.Errorf("Clean(%s) got %q, want %q", c.in, got, want)
		}
	}
}

func TestEncodeDecode(t *testing.T) {
	cases := []struct {
		unenc, enc string
	}{
		{"", ""},
		{"/", "%2F"},
		{"%", "%25"},
		{"/The % rain in /% Spain", "%2FThe %25 rain in %2F%25 Spain"},
		{"/%/%", "%2F%25%2F%25"},
		{"ᚸӲ읔קAل", "ᚸӲ읔קAل"},
		{"ᚸ/Ӳ%읔/ק%A+ل", "ᚸ%2FӲ%25읔%2Fק%25A+ل"},
	}
	for _, c := range cases {
		if want, got := c.enc, EncodeAsNameElement(c.unenc); got != want {
			t.Errorf("EncodeAsNameElement(%s) got %q, want %q", c.unenc, got, want)
		}
	}
	for _, c := range cases {
		want := c.unenc
		got, ok := DecodeFromNameElement(c.enc)
		if !ok {
			t.Errorf("DecodeFromNameElement(%s) not ok", c.enc)
		}
		if got != want {
			t.Errorf("DecodeFromNameElement(%s) got %q, want %q", c.enc, got, want)
		}
	}
	badEncodings := []string{"%2", "%aqq"}
	for _, bad := range badEncodings {
		want := bad
		got, ok := DecodeFromNameElement(bad)
		if ok {
			t.Errorf("DecodeFromNameElement(%s) should not be ok", bad)
		}
		if got != want {
			t.Errorf("DecodeFromNameElement(%s) got %q, want %q", bad, got, want)

		}
	}
}
