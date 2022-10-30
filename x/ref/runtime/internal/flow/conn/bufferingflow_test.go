// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"io"
	"sync"
	"testing"

	"v.io/v23/flow"
	"v.io/x/ref/test"
)

func TestBufferingFlow(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	flows, accept, dc, ac := setupFlowsOpts(t, "local", "", ctx, ctx, true, 1, Opts{MTU: 16})

	errCh := make(chan error, 2)
	msgCh := make(chan []byte, 10)
	var wg sync.WaitGroup
	wg.Add(2)

	bf := NewBufferingFlow(ctx, flows[0])

	go func(f flow.Flow) {
		defer wg.Done()
		for i, m := range []string{"hello",
			" there",
			" world",
			" some more that ...should cause a flush"} {
			n, err := f.WriteMsg([]byte(m))
			if err != nil {
				errCh <- err
				return
			}
			if got, want := n, len(m); got != want {
				errCh <- fmt.Errorf("%v: got %v, want %v (%s)", i, got, want, m)
			}
		}
		f.Close()
	}(bf)

	go func(f flow.Flow) {
		defer wg.Done()
		for {
			m, err := f.ReadMsg()
			if err != nil {
				if err != io.EOF {
					errCh <- err
				}
				return
			}
			msgCh <- m
		}
	}(<-accept)

	wg.Wait()
	close(errCh)
	close(msgCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	expect := []string{
		"hello there",
		" world",
		" some more that ",
		"...should cause ",
		"a flush",
	}
	i := 0
	for msg := range msgCh {
		if got, want := string(msg), expect[i]; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		i++
	}

	ac.Close(ctx, nil)
	dc.Close(ctx, nil)
	<-ac.Closed()
	<-dc.Closed()
}
