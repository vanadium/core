// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"testing"

	"v.io/v23/context"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

type readqRelease struct {
	n int
}

func (rr *readqRelease) release(ctx *context.T, n int) {
	rr.n += n
}

func readqPut(ctx *context.T, r *readq, m string) {
	nb, b := getNetBuf(len(m))
	b = append(b[:0], []byte(m)...)
	r.put(ctx, b, nb)
}

func TestReadqRead(t *testing.T) {
	defer goroutines.NoLeaks(t, 0)()
	defer netbufsFreed(t)

	ctx, shutdown := test.V23Init()
	defer shutdown()

	rr := &readqRelease{}

	r := newReadQ(DefaultBytesBuffered, rr.release)

	put := func(m string) { readqPut(ctx, r, m) }

	put("one")
	put("two")
	put("thre")
	put("reallong")
	r.close(ctx)

	read := make([]byte, 4)
	want := []string{"one", "two", "thre", "real", "long"}
	for _, w := range want {
		n, err := r.read(ctx, read)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := string(read[:n]); got != w {
			t.Errorf("got: %s, want %s", got, w)
		}
	}
	if _, err := r.read(ctx, read); err != io.EOF {
		t.Errorf("expected EOF got %v", err)
	}
}

func TestReadqGet(t *testing.T) {
	defer goroutines.NoLeaks(t, 0)()
	defer netbufsFreed(t)

	ctx, shutdown := test.V23Init()
	defer shutdown()

	rr := &readqRelease{}

	r := newReadQ(DefaultBytesBuffered, rr.release)
	put := func(m string) { readqPut(ctx, r, m) }
	put("one")
	put("two")
	put("thre")
	put("reallong")
	r.close(ctx)

	want := []string{"one", "two", "thre", "reallong"}
	for _, w := range want {
		out, err := r.get(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := string(out); got != w {
			t.Errorf("got: %s, want %s", got, w)
		}
	}
	if _, err := r.get(ctx); err != io.EOF {
		t.Errorf("expected EOF got %v", err)
	}
}

func TestReadqMixed(t *testing.T) {
	defer goroutines.NoLeaks(t, 0)()
	defer netbufsFreed(t)

	ctx, shutdown := test.V23Init()
	defer shutdown()

	rr := &readqRelease{}

	r := newReadQ(DefaultBytesBuffered, rr.release)
	put := func(m string) { readqPut(ctx, r, m) }
	put("one")
	put("two")
	put("thre")
	put("reallong")
	r.close(ctx)

	want := []string{"one", "two", "thre", "real", "long"}
	for i, w := range want {
		var (
			err  error
			got  string
			n    int
			out  []byte
			read = make([]byte, 4)
		)
		if i%2 == 0 {
			out, err = r.get(ctx)
			got = string(out)
		} else {
			n, err = r.read(ctx, read)
			got = string(read[:n])
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != w {
			t.Errorf("got: %s, want %s", got, w)
		}
	}
	if _, err := r.get(ctx); err != io.EOF {
		t.Errorf("expected EOF got %v", err)
	}
	if _, err := r.read(ctx, nil); err != io.EOF {
		t.Errorf("expected EOF got %v", err)
	}

	if got, want := rr.n, 18; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReadqQResize(t *testing.T) {
	defer netbufsFreed(t)

	ctx, shutdown := test.V23Init()
	defer shutdown()

	rr := &readqRelease{}

	r := newReadQ(DefaultBytesBuffered, rr.release)

	for i := 0; i < 100; i++ {
		readqPut(ctx, r, fmt.Sprintf("%03v", i))
	}

	if got, want := r.nbufs, 100; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	for i := 0; i < 100; i++ {
		msg, _ := r.get(ctx)
		if got, want := msg, []byte(fmt.Sprintf("%03v", i)); !bytes.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	if got, want := cap(r.bufs), 160; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	r.put(ctx, []byte(fmt.Sprintf("%03v", 0)), nil)
	if got, want := cap(r.bufs), 40; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	for i := 1; i < 100; i++ {
		readqPut(ctx, r, fmt.Sprintf("%03v", i))
	}

	for i := 0; i < 100; i++ {
		buf := [10]byte{}
		n, _ := r.read(ctx, buf[:])
		if got, want := buf[:n], []byte(fmt.Sprintf("%03v", i)); !bytes.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestReadqClose(t *testing.T) {
	defer goroutines.NoLeaks(t, 0)()
	defer netbufsFreed(t)

	ctx, shutdown := test.V23Init()
	defer shutdown()

	rr := &readqRelease{}

	r := newReadQ(DefaultBytesBuffered, rr.release)

	iterations := 1000
	var wg sync.WaitGroup
	wg.Add(2)
	ch := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			readqPut(ctx, r, fmt.Sprintf("%03v", i))
		}
	}()

	go func() {
		defer wg.Done()
		stopat := rand.Int31n(int32(iterations)) //nolint: gosec
		for i := 0; i < iterations; i++ {
			msg, err := r.get(ctx)
			if err == io.EOF {
				break
			}
			if got, want := msg, []byte(fmt.Sprintf("%03v", i)); !bytes.Equal(got, want) {
				errCh <- fmt.Errorf("got %s, want %s: %v", got, want, err)
				return
			}
			if i == int(stopat) {
				close(ch)
			}
		}
		errCh <- nil
	}()

	<-ch
	r.close(ctx)

	wg.Wait()
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}
