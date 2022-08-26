// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

func mkBufs(in ...string) [][]byte {
	out := make([][]byte, len(in))
	for i, s := range in {
		out[i] = []byte(s)
	}
	return out
}

func TestReadqRead(t *testing.T) {
	defer goroutines.NoLeaks(t, 0)()

	ctx, shutdown := test.V23Init()
	defer shutdown()

	r := newReadQ(nil, 1)
	r.put(ctx, mkBufs("one", "two"))
	r.put(ctx, mkBufs("thre", "reallong"))
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

func mkBufsRand(sizes ...int) [][]byte {
	out := make([][]byte, len(sizes))
	for i, s := range sizes {
		out[i] = make([]byte, s)
		io.ReadFull(rand.Reader, out[i])
	}
	return out
}

func TestReadqReadRandom(t *testing.T) {
	defer goroutines.NoLeaks(t, 0)()

	ctx, shutdown := test.V23Init()
	defer shutdown()

	r := newReadQ(nil, 1)
	bufs := mkBufsRand(100, 1024, 4096, 8192, 8192, 8192, 100*1024)
	r.put(ctx, bufs)
	r.close(ctx)

	for _, want := range bufs {
		read := make([]byte, len(want))
		n, err := r.read(ctx, read)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := read[:n]; !bytes.Equal(got, want) {
			t.Errorf("got: %s, want %s", got[:10], want[:10])
		}
	}
	if _, err := r.read(ctx, []byte{}); err != io.EOF {
		t.Errorf("expected EOF got %v", err)
	}
}

func TestReadqGetRandom(t *testing.T) {
	defer goroutines.NoLeaks(t, 0)()

	ctx, shutdown := test.V23Init()
	defer shutdown()

	r := newReadQ(nil, 1)
	bufs := mkBufsRand(100, 1024, 4096, 8192, 8192, 8192, 100*1024)
	r.put(ctx, bufs)
	r.close(ctx)

	for _, want := range bufs {
		read, err := r.get(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := read; !bytes.Equal(got, want) {
			t.Errorf("got: %s, want %s", got[:10], want[:10])
		}
	}
	if _, err := r.get(ctx); err != io.EOF {
		t.Errorf("expected EOF got %v", err)
	}
}

func TestReadqGet(t *testing.T) {
	defer goroutines.NoLeaks(t, 0)()

	ctx, shutdown := test.V23Init()
	defer shutdown()

	r := newReadQ(nil, 1)
	r.put(ctx, mkBufs("one", "two"))       //nolint:errcheck
	r.put(ctx, mkBufs("thre", "reallong")) //nolint:errcheck
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

	ctx, shutdown := test.V23Init()
	defer shutdown()

	r := newReadQ(nil, 1)
	r.put(ctx, mkBufs("one", "two"))       //nolint:errcheck
	r.put(ctx, mkBufs("thre", "reallong")) //nolint:errcheck
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
}
