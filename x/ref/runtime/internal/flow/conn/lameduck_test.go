// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

func TestLameDuck(t *testing.T) { //nolint:gocyclo
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()

	dflows, aflows := make(chan flow.Flow, 3), make(chan flow.Flow, 3)
	dc, ac, derr, aerr := setupConns(t, "local", "", ctx, ctx, dflows, aflows, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}

	go func() {
		for {
			select {
			case f := <-aflows:
				if got, err := f.ReadMsg(); err != nil {
					panic(fmt.Sprintf("got %v wanted nil", err))
				} else if !bytes.Equal(got, []byte("hello")) {
					panic(fmt.Sprintf("got %q, wanted 'hello'", string(got)))
				}
			case <-ac.Closed():
				return
			}
		}
	}()

	// Dial a flow and write it (which causes it to open).
	f1, err := dc.Dial(ctx, dc.LocalBlessings(), nil, naming.Endpoint{}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f1.WriteMsg([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	// Dial more flows, but don't write to them yet.
	f2, err := dc.Dial(ctx, dc.LocalBlessings(), nil, naming.Endpoint{}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	f3, err := dc.Dial(ctx, dc.LocalBlessings(), nil, naming.Endpoint{}, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	// Now put the accepted conn into lame duck mode and wait for the dialed
	// conn to get the message.
	ldch := ac.EnterLameDuck(ctx)
	waitFor(dc.RemoteLameDuck)

	// Now we shouldn't be able to dial from dc because it's in lame duck mode.
	if _, err := dc.Dial(ctx, dc.LocalBlessings(), nil, naming.Endpoint{}, 0, false); err == nil {
		t.Fatalf("expected an error, got nil")
	}

	// I can't think of a non-flaky way to test for it, but it should
	// be the case that we don't send the AckLameDuck message until
	// we write to or close the other flows.  This should catch it sometimes.
	time.Sleep(time.Millisecond * 100)
	if ac.Status() == LameDuckAcknowledged {
		t.Errorf("Didn't expect the acceptor to see a lame duck ack yet.")
	}

	// Now write or close the other flows.
	if _, err := f2.WriteMsg([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	f3.Close()

	// Now the acceptor should enter LameDuckAcknowledged.
	<-ldch
	if status := ac.Status(); status != LameDuckAcknowledged {
		t.Errorf("Got %d, wanted %d.", status, LameDuckAcknowledged)
	}

	// Now put the dialer side into lame duck.
	ldch = dc.EnterLameDuck(ctx)
	waitFor(ac.RemoteLameDuck)
	<-ldch
	if status := dc.Status(); status != LameDuckAcknowledged {
		t.Errorf("Got %d, wanted %d.", status, LameDuckAcknowledged)
	}

	// Now close the accept side.
	ac.Close(ctx, nil)
	<-dc.Closed()
	<-ac.Closed()
	if status := dc.Status(); status != Closed {
		t.Errorf("got %d, want %d", status, Closed)
	}
	if status := ac.Status(); status != Closed {
		t.Errorf("got %d, want %d", status, Closed)
	}
}
