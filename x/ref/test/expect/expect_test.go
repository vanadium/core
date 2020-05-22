// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package expect_test

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/x/ref/test/expect"
)

func TestSimple(t *testing.T) {
	buf := []byte{}
	buffer := bytes.NewBuffer(buf)
	buffer.WriteString("bar\n")
	buffer.WriteString("baz\n")
	buffer.WriteString("oops\n")
	s := expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	s.Expect("bar")
	s.Expect("baz")
	if err := s.Error(); err != nil {
		t.Error(err)
	}
	// This will fail the test.
	s.Expect("not oops")
	if err := s.Error(); err == nil {
		t.Error("unexpected success")
	} else {
		t.Log(s.Error())
	}
}

func TestExpectf(t *testing.T) {
	buf := []byte{}
	buffer := bytes.NewBuffer(buf)
	buffer.WriteString("bar 22\n")
	s := expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	s.Expectf("bar %d", 22)
	s.ExpectEOF() //nolint:errcheck
	if err := s.Error(); err != nil {
		t.Error(err)
	}
}

func TestEOF(t *testing.T) {
	buf := []byte{}
	buffer := bytes.NewBuffer(buf)
	buffer.WriteString("bar 22\n")
	buffer.WriteString("baz 22\n")
	s := expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	s.Expectf("bar %d", 22)
	s.ExpectEOF() //nolint:errcheck
	if err := s.Error(); err == nil {
		t.Error("unexpected success")
	} else {
		t.Log(s.Error())
	}
}

func TestExpectRE(t *testing.T) {
	buf := []byte{}
	buffer := bytes.NewBuffer(buf)
	buffer.WriteString("bar=baz\n")
	buffer.WriteString("aaa\n")
	buffer.WriteString("bbb\n")
	s := expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	if got, want := s.ExpectVar("bar"), "baz"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	s.ExpectRE("zzz|aaa", -1)
	if err := s.Error(); err != nil {
		t.Error(err)
	}
	if got, want := s.ExpectRE("(.*)", -1), [][]string{{"bbb", "bbb"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := s.ExpectRE("(.*", -1), [][]string{{"bbb", "bbb"}}; !reflect.DeepEqual(got, want) {
		// this will have failed the test also.
		if err := s.Error(); err == nil || !strings.Contains(err.Error(), "error parsing regexp") {
			t.Errorf("missing or wrong error: %v", s.Error())
		}
	}
}

func TestExpectSetRE(t *testing.T) {
	buf := []byte{}
	buffer := bytes.NewBuffer(buf)
	buffer.WriteString("bar=baz\n")
	buffer.WriteString("abc\n")
	buffer.WriteString("def\n")
	buffer.WriteString("abc\n")
	s := expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	got := s.ExpectSetRE("^bar=.*$", "def$", "^abc$", "^a..$")
	if s.Error() != nil {
		t.Errorf("unexpected error: %s", s.Error())
	}
	want := [][]string{{"bar=baz"}, {"def"}, {"abc"}, {"abc"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected result from ExpectSetRE, got %v, want %v", got, want)
	}
	buffer.WriteString("ooh\n")
	buffer.WriteString("aah\n")
	s.ExpectSetRE("bar=.*", "def")
	if got, want := s.Error(), "found no match for [bar=.*,def]"; got == nil || !strings.Contains(got.Error(), want) {
		t.Errorf("got %v, wanted something containing %q", got, want)
	}

	buf = []byte{}
	buffer = bytes.NewBuffer(buf)
	s = expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	buffer.WriteString("hello world\n")
	buffer.WriteString("this is a test\n")
	matches := s.ExpectSetRE("hello (world)", "this (is) (a|b) test")
	if want := [][]string{{"hello world", "world"}, {"this is a test", "is", "a"}}; !reflect.DeepEqual(want, matches) {
		t.Errorf("unexpected result from ExpectSetRE, got %v, want %v", matches, want)
	}

	buf = []byte{}
	buffer = bytes.NewBuffer(buf)
	s = expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	buffer.WriteString("aaa\n")
	buffer.WriteString("aaa\n")
	buffer.WriteString("aaa\n")

	// Expect 3 x aaa to match.
	s.ExpectSetRE("aaa", "aaa", "aaa")
	if s.Error() != nil {
		t.Errorf("unexpected error: %v", s.Error())
	}

	// Expecting one more aaa should fail: the entire input should have been consumed.
	s.ExpectSetRE("aaa")
	if s.Error() == nil {
		t.Errorf("expected error but got none")
	}

	// Test a buffer that contains a match but not within the number of lines we expect.
	buf = []byte{}
	buffer = bytes.NewBuffer(buf)
	buffer.WriteString("aaa\n")
	buffer.WriteString("bbb\n")
	s = expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	s.ExpectSetRE("bbb")
	if s.Error() == nil {
		t.Fatalf("expected error but got none")
	}

	// Test a buffer that contains a match and leaves us with nothing more to read.
	buf = []byte{}
	buffer = bytes.NewBuffer(buf)
	buffer.WriteString("aaa\n")
	buffer.WriteString("bbb\n")
	s = expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	s.ExpectSetRE("bbb")
	if s.Error() == nil {
		t.Fatalf("expected error but got none")
	}

	// Now ensure that each regular expression matches a unique line.
	buf = []byte{}
	buffer = bytes.NewBuffer(buf)
	buffer.WriteString("a 1\n")
	buffer.WriteString("a 2\n")
	buffer.WriteString("a 3\n")
	s = expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	matches = s.ExpectSetRE("\\w (\\d)", "a (\\d)", "a (\\d)")
	want = [][]string{{"a 1", "1"}, {"a 2", "2"}, {"a 3", "3"}}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("unexpected result from ExpectSetRE, got %v, want %v", matches, want)
	}
	if s.ExpectEOF() != nil {
		t.Fatalf("expected EOF but did not get it")
	}
}

func TestExpectSetEventuallyRE(t *testing.T) {
	buf := []byte{}
	buffer := bytes.NewBuffer(buf)
	buffer.WriteString("bar=baz\n")
	buffer.WriteString("abc\n")
	buffer.WriteString("def\n")
	buffer.WriteString("abc\n")
	s := expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	s.SetVerbosity(testing.Verbose())
	s.ExpectSetEventuallyRE("^bar=.*$", "def")
	if s.Error() != nil {
		t.Errorf("unexpected error: %s", s.Error())
	}

	// Should see one more abc match after the we read def.
	s.ExpectSetEventuallyRE("abc")
	if s.Error() != nil {
		t.Errorf("unexpected error: %s", s.Error())
	}

	// Trying to match abc again should yield an error.
	s.ExpectSetEventuallyRE("abc")
	if got, want := s.Error(), "found no match for [abc]"; got == nil || !strings.Contains(got.Error(), want) {
		t.Errorf("got %q, wanted something containing %q", got, want)
	}

	// Need to clear the EOF from the previous ExpectSetEventuallyRE call
	buf = []byte{}
	buffer = bytes.NewBuffer(buf)
	s = expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	buffer.WriteString("ooh\n")
	buffer.WriteString("aah\n")
	s.ExpectSetEventuallyRE("zzz")
	if got, want := s.Error(), "found no match for [zzz]"; got == nil || !strings.Contains(got.Error(), want) {
		t.Errorf("got %q, wanted something containing %q", got, want)
	}

	buf = []byte{}
	buffer = bytes.NewBuffer(buf)
	s = expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	buffer.WriteString("not expected\n")
	buffer.WriteString("hello world\n")
	buffer.WriteString("this is a test\n")
	matches := s.ExpectSetEventuallyRE("hello (world)", "this (is) (a|b) test")
	if want := [][]string{{"hello world", "world"}, {"this is a test", "is", "a"}}; !reflect.DeepEqual(want, matches) {
		t.Errorf("unexpected result from ExpectSetRE, got %v, want %v", matches, want)
	}

	// Test error output with multiple unmatched res.
	buf = []byte{}
	buffer = bytes.NewBuffer(buf)
	s = expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	buffer.WriteString("not expected\n")
	buffer.WriteString("hello world\n")
	buffer.WriteString("this is a test\n")
	s.ExpectSetEventuallyRE("blargh", "blerg", "blorg")
	if got, want := s.Error(), "found no match for [blargh,blerg,blorg]"; !strings.Contains(got.Error(), want) {
		t.Errorf("got %q, wanted something containing %q", got, want)
	}
}

func TestRead(t *testing.T) {
	buf := []byte{}
	buffer := bytes.NewBuffer(buf)
	lines := []string{"some words", "bar=baz", "more words"}
	for _, l := range lines {
		buffer.WriteString(l + "\n")
	}
	s := expect.NewSession(nil, bufio.NewReader(buffer), time.Minute)
	for _, l := range lines {
		if got, want := s.ReadLine(), l; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	if s.Failed() {
		t.Errorf("unexpected error: %s", s.Error())
	}
	want := ""
	for i := 0; i < 100; i++ {
		m := fmt.Sprintf("%d\n", i)
		buffer.WriteString(m)
		want += m
	}
	got, err := s.ReadAll()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if s.ExpectEOF() != nil {
		t.Fatalf("expected EOF but did not get it")
	}
}
