// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"v.io/v23/verror"

	"v.io/v23/context"
	"v.io/v23/flow"
	_ "v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

const leakWaitTime = 500 * time.Millisecond

var randData []byte

func init() {
	randData = make([]byte, 2*DefaultBytesBufferedPerFlow())
	if _, err := rand.Read(randData); err != nil {
		panic("Could not read random data.")
	}
}

func trunc(b []byte) []byte {
	if len(b) > 100 {
		return b[:100]
	}
	return b
}

func doWrite(f flow.Flow, data []byte) error {
	mid := len(data) / 2
	wrote, err := f.WriteMsg(data[:mid], data[mid:])
	if err != nil || wrote != len(data) {
		return fmt.Errorf("Unexpected result for write: %d, %v wanted %d, nil", wrote, err, len(data))
	}
	return nil
}

func doRead(f flow.Flow, want []byte, wg *sync.WaitGroup) error {
	for read := 0; len(want) > 0; read++ {
		got, err := f.ReadMsg()
		if err != nil && err != io.EOF {
			return fmt.Errorf("Unexpected error: %v", err)
		}
		if !bytes.Equal(got, want[:len(got)]) {
			return fmt.Errorf("On read %d got: %v want %v", read, trunc(got), trunc(want))
		}
		want = want[len(got):]
	}
	if len(want) != 0 {
		return fmt.Errorf("got %d leftover bytes, expected 0.", len(want))
	}
	if wg != nil {
		wg.Done()
	}
	return nil
}

func TestLargeWrite(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()
	df, flows, cl := setupFlow(t, "local", "", ctx, ctx, true)
	defer cl()

	errs := make(chan error, 4)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		errs <- doWrite(df, randData)
	}()
	go func() {
		errs <- doRead(df, randData, &wg)
	}()
	af := <-flows
	go func() {
		errs <- doRead(af, randData, &wg)
	}()
	go func() {
		errs <- doWrite(af, randData)
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestManyLargeWrites(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()
	df, flows, cl := setupFlow(t, "local", "", ctx, ctx, true)
	defer cl()

	iterations := 200
	errs := make(chan error, iterations*44)
	var wg sync.WaitGroup
	wg.Add(2)

	writer := func(f flow.Flow) {
		for i := 0; i < iterations; i++ {
			errs <- doWrite(f, randData)
		}
	}
	reader := func(f flow.Flow) {
		for i := 0; i < iterations; i++ {
			errs <- doRead(f, randData, nil)
		}
		wg.Done()
	}

	go writer(df)
	go reader(df)

	af := <-flows

	go reader(af)
	go writer(af)

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestConnRTT(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	payload := []byte{0}
	ctx, shutdown := test.V23Init()
	defer shutdown()
	df, flows, cl := setupFlow(t, "local", "", ctx, ctx, true)
	defer cl()

	errs := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		errs <- doWrite(df, payload)
		wg.Done()
	}()
	af := <-flows

	if df.Conn().RTT() == 0 {
		t.Errorf("dialed conn's RTT should be non-zero")
	}
	if af.Conn().RTT() == 0 {
		t.Errorf("accepted conn's RTT should be non-zero")
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestMinChannelTimeout(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	orig := minChannelTimeout
	defer func() {
		minChannelTimeout = orig
	}()
	minChannelTimeout = map[string]time.Duration{
		"local": time.Minute,
	}

	ctx, shutdown := test.V23Init()
	defer shutdown()
	dflows, aflows := make(chan flow.Flow, 1), make(chan flow.Flow, 1)
	dc, ac, derr, aerr := setupConns(t, "local", "", ctx, ctx, dflows, aflows, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}
	defer dc.Close(ctx, nil)
	defer ac.Close(ctx, nil)

	if err := deadlineInAbout(dc, defaultChannelTimeout); err != nil {
		t.Error(err)
	}
	if err := deadlineInAbout(ac, defaultChannelTimeout); err != nil {
		t.Error(err)
	}

	df, af := oneFlow(t, ctx, dc, aflows, 0)
	if err := deadlineInAbout(dc, defaultChannelTimeout); err != nil {
		t.Error(err)
	}
	if err := deadlineInAbout(ac, defaultChannelTimeout); err != nil {
		t.Error(err)
	}
	df.Close()
	af.Close()

	df, af = oneFlow(t, ctx, dc, aflows, 10*time.Minute)
	if err := deadlineInAbout(dc, 10*time.Minute); err != nil {
		t.Error(err)
	}
	if err := deadlineInAbout(ac, defaultChannelTimeout); err != nil {
		t.Error(err)
	}
	df.Close()
	af.Close()

	// Here the min timeout will come into play.
	df2, af2 := oneFlow(t, ctx, dc, aflows, time.Second)
	if err := deadlineInAbout(dc, time.Minute); err != nil {
		t.Error(err)
	}
	if err := deadlineInAbout(ac, defaultChannelTimeout); err != nil {
		t.Error(err)
	}
	df2.Close()
	af2.Close()

	// Setup new conns with a default channel timeout below the min.
	dc, ac, derr, aerr = setupConnsWithTimeout(t, "local", "", ctx, ctx, dflows, aflows, nil, nil, 0, time.Minute, time.Second, DefaultBytesBufferedPerFlow())
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}
	defer dc.Close(ctx, nil)
	defer ac.Close(ctx, nil)
	// They should both start with the min value.
	if err := deadlineInAbout(dc, time.Minute); err != nil {
		t.Error(err)
	}
	if err := deadlineInAbout(ac, time.Minute); err != nil {
		t.Error(err)
	}
}

func deadlineInAbout(c *Conn, d time.Duration) error {
	const slop = 5 * time.Second
	delta := time.Until(c.healthCheckCloseDeadline())
	if delta > d-slop && delta < d+slop {
		return nil
	}
	return fmt.Errorf("got %v want %v (+-5s)", delta, d)
}

func TestHandshakeDespiteCancel(t *testing.T) {
	// This test is specifically for the race documented in:
	// https://github.com/vanadium/core/issues/40
	// Even though the dial context is cancelled the handshake should
	// complete, but returning an error that indicating that it
	// was canceled.
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()

	dctx, dcancel := context.WithTimeout(ctx, time.Minute)
	dflows, aflows := make(chan flow.Flow, 1), make(chan flow.Flow, 1)
	dcancel()
	dc, ac, derr, aerr := setupConnsWithTimeout(t, "local", "", dctx, ctx, dflows, aflows, nil, nil, 1*time.Second, 4*time.Second, time.Second, DefaultBytesBufferedPerFlow())
	if aerr != nil {
		t.Fatalf("setupConnsWithTimeout: unexpectedly failed: %v, %v", derr, aerr)
	}
	if got, want := verror.ErrorID(derr), verror.ErrCanceled.ID; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	dc.Close(ctx, nil)
	ac.Close(ctx, nil)
}
