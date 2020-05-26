// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package drrqueue

import (
	"log"
	"runtime"
	"testing"

	"v.io/x/ref/runtime/internal/lib/bqueue"
	"v.io/x/ref/runtime/internal/lib/iobuf"
)

const (
	testQuantum = 1 << 14 // 16K
)

// concat concatenates the buffers into a string.
func concat(bufs []*iobuf.Slice) string {
	buf := []byte{}
	for _, b := range bufs {
		buf = append(buf, b.Contents...)
		b.Release()
	}
	return string(buf)
}

// mkbufs makes a iobuf.Slice from a string.
func mkbufs(s string) *iobuf.Slice {
	return iobuf.NewSlice([]byte(s))
}

// makeBuffer makes a byte buffer filled with the initial char.
func makeBuffer(size int, c byte) string {
	b := make([]byte, size)
	for i := 0; i != size; i++ {
		b[i] = c
	}
	return string(b)
}

// A "reader" copies data from the q to a string channel.
func startReader(q bqueue.T) chan string {
	c := make(chan string)
	go func() {
		for {
			_, bufs, err := q.Get(nil)
			if err != nil {
				log.Printf("Reader: %s", err)
				break
			}
			c <- concat(bufs)
		}
		c <- "DONE"
	}()
	return c
}

// expectedGet compares the sequence returned from q.Get() against a sequence
// of expected strings.
func expectGet(t *testing.T, q bqueue.T, strings []string) {
	_, file, line, _ := runtime.Caller(1)
	for _, s1 := range strings {
		_, buf, err := q.Get(nil)
		if err != nil {
			t.Errorf("%s(%d): Unexpected error: %v", file, line, err)
			break
		}
		s2 := concat(buf)
		if s2 != s1 {
			t.Errorf("%s(%d): Expected %q, but received %q", file, line, s1, s2)
		}
	}
}

// TestSimple tests a Put/Release/Get sequence.
func TestSimple(t *testing.T) {
	q := New(testQuantum)
	w, _ := q.NewWriter(0, 0, 5)
	log.Printf("PutV")
	w.Put(mkbufs("Hello"), nil) //nolint:errcheck
	log.Printf("Release")
	w.Release(5) //nolint:errcheck
	log.Printf("Get")
	w2, buf, err := q.Get(nil)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	s := concat(buf)
	if s != "Hello" {
		t.Errorf("Expected 'Hello', received %q", s)
	}
	if w2 != w {
		t.Errorf("Writer mistmatch")
	}
}

func TestShutdownWithoutRemove(t *testing.T) {
	q := New(testQuantum)
	w1, _ := q.NewWriter(0, 0, 100)
	w2, _ := q.NewWriter(1, 1, 100)

	w1.Put(mkbufs("1_1"), nil) //nolint:errcheck
	w1.Put(mkbufs("1_2"), nil) //nolint:errcheck
	w2.Put(mkbufs("2_1"), nil) //nolint:errcheck
	w2.Put(mkbufs("2_2"), nil) //nolint:errcheck

	w1.Release(3) //nolint:errcheck
	w2.Release(3) //nolint:errcheck

	w, buf, err := q.Get(nil)
	if s := concat(buf); err != nil || w.ID() != w1.ID() || s != "1_1" {
		t.Errorf("Expected '1_1' from 0 with nil error, found %s from %d with %v", s, w.ID(), err)
	}

	w1.Shutdown(false)

	w, buf, err = q.Get(nil)
	if s := concat(buf); err != nil || w.ID() != w1.ID() || s != "" {
		t.Errorf("Expected '' from 0 with nil error, found %s from %d with %v", s, w.ID(), err)
	}

	// Now we have to remove the writer from q.
	w1.Shutdown(true)

	w, buf, err = q.Get(nil)
	if s := concat(buf); err != nil || w.ID() != w2.ID() || s != "2_1" {
		t.Errorf("Expected '2_1' from 1 with nil error, found %s from %d with %v", s, w.ID(), err)
	}
}

// TestRelease tests whether data is released in Release() order.
func TestRelease(t *testing.T) {
	q := New(testQuantum)
	c := startReader(q)
	w1, _ := q.NewWriter(0, 0, 10)
	w2, _ := q.NewWriter(1, 0, 10)
	w1.Put(mkbufs("A1"), nil) //nolint:errcheck
	w1.Put(mkbufs("A2"), nil) //nolint:errcheck
	w2.Put(mkbufs("B1"), nil) //nolint:errcheck
	w2.Put(mkbufs("B2"), nil) //nolint:errcheck
	select {
	case s := <-c:
		t.Errorf("Unexpected Get: %q", s)
	default:
	}

	w2.Release(1) //nolint:errcheck
	select {
	case s := <-c:
		t.Errorf("Expected no release, but received %q", s)
	default:
	}

	w2.Release(1) //nolint:errcheck
	s := <-c
	if s != "B1" {
		t.Errorf("Expected 'B1', but received %q", s)
	}

	w1.Release(4) //nolint:errcheck
	s = <-c
	if s != "A1A2" {
		t.Errorf("Expected 'A1', but received %q", s)
	}

	w1.Release(2) //nolint:errcheck
	select {
	case s := <-c:
		t.Errorf("Unexpected Get: %q", s)
	default:
	}

	w1.Put(mkbufs("A3"), nil) //nolint:errcheck
	s = <-c
	if s != "A3" {
		t.Errorf("Expected 'A3', but received %q", s)
	}

	w2.Release(2) //nolint:errcheck
	s = <-c
	if s != "B2" {
		t.Errorf("Expected 'B2', but received %q", s)
	}

	select {
	case s := <-c:
		t.Errorf("Unexpected Get: %q", s)
	default:
	}

	q.Close()
	s = <-c
	if s != "DONE" {
		t.Errorf("Expected 'DONE', but received %q", s)
	}
}

// TestPriority tests the priority.
func TestPriority(t *testing.T) {
	q := New(testQuantum)
	w1, _ := q.NewWriter(0, 1, 100)
	w2, _ := q.NewWriter(1, 0, 100)
	w1.Release(100) //nolint:errcheck

	w2.Release(100) //nolint:errcheck

	w1.Put(mkbufs("a"), nil) //nolint:errcheck
	w1.Put(mkbufs("b"), nil) //nolint:errcheck
	w2.Put(mkbufs("c"), nil) //nolint:errcheck
	w2.Put(mkbufs("d"), nil) //nolint:errcheck

	expectGet(t, q, []string{"cd", "ab"})

	w1.Put(mkbufs("a"), nil) //nolint:errcheck
	w1.Put(mkbufs("b"), nil) //nolint:errcheck
	w2.Put(mkbufs("c"), nil) //nolint:errcheck
	w2.Put(mkbufs("d"), nil) //nolint:errcheck

	expectGet(t, q, []string{"cd", "ab"})
}

// TestRoundRobin tests the round robin policy.
func TestRoundRobin(t *testing.T) {
	q := New(testQuantum)
	w1, _ := q.NewWriter(0, 0, 100)
	w2, _ := q.NewWriter(1, 0, 100)
	w1.Release(100) //nolint:errcheck
	w2.Release(100) //nolint:errcheck

	w1.Put(mkbufs("a"), nil) //nolint:errcheck
	w1.Put(mkbufs("b"), nil) //nolint:errcheck
	w2.Put(mkbufs("c"), nil) //nolint:errcheck
	w2.Put(mkbufs("d"), nil) //nolint:errcheck
	expectGet(t, q, []string{"ab", "cd"})

	w2.Put(mkbufs("a"), nil) //nolint:errcheck
	w1.Put(mkbufs("b"), nil) //nolint:errcheck
	w2.Put(mkbufs("c"), nil) //nolint:errcheck
	w1.Put(mkbufs("d"), nil) //nolint:errcheck
	w1.Put(mkbufs("e"), nil) //nolint:errcheck
	expectGet(t, q, []string{"ac", "bde"})
}

// TestDeficit tests the deficit counter.
func TestDeficit(t *testing.T) {
	q := New(testQuantum)
	w1, _ := q.NewWriter(0, 0, testQuantum*10)
	w2, _ := q.NewWriter(1, 0, testQuantum*10)
	w1.Release(-1) //nolint:errcheck
	w2.Release(-1) //nolint:errcheck

	b1a := makeBuffer(2*testQuantum, '1')
	b1b := makeBuffer(2*testQuantum, '2')
	b2a := makeBuffer(testQuantum, '3')
	b2b := makeBuffer(testQuantum, '4')
	b2c := makeBuffer(testQuantum, '5')
	b2d := makeBuffer(testQuantum, '6')
	b2e := makeBuffer(testQuantum, '7')
	w1.Put(mkbufs(b1a), nil) //nolint:errcheck
	w1.Put(mkbufs(b1b), nil) //nolint:errcheck
	w2.Put(mkbufs(b2a), nil) //nolint:errcheck
	w2.Put(mkbufs(b2b), nil) //nolint:errcheck
	w2.Put(mkbufs(b2c), nil) //nolint:errcheck
	w2.Put(mkbufs(b2d), nil) //nolint:errcheck
	w2.Put(mkbufs(b2e), nil) //nolint:errcheck
	expectGet(t, q, []string{b2a, b1a, b2b, b2c, b1b, b2d, b2e})
}
