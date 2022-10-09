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

	v23 "v.io/v23"
	"v.io/v23/naming"
	"v.io/v23/verror"

	"v.io/v23/context"
	"v.io/v23/flow"
	_ "v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/runtime/internal/flow/flowtest"
	"v.io/x/ref/runtime/internal/rpc/version"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

const leakWaitTime = 500 * time.Millisecond

var randData []byte

func init() {
	randData = make([]byte, 2*DefaultBytesBuffered)
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
		return fmt.Errorf("got %d leftover bytes, expected 0", len(want))
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
	wg.Add(4)
	go func() {
		errs <- doWrite(df, randData)
		wg.Done()
	}()
	go func() {
		errs <- doRead(df, randData, nil)
		wg.Done()
	}()
	af := <-flows
	go func() {
		errs <- doRead(af, randData, nil)
		wg.Done()
	}()
	go func() {
		errs <- doWrite(af, randData)
		wg.Done()
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
	wg.Add(4)

	writer := func(f flow.Flow) {
		for i := 0; i < iterations; i++ {
			errs <- doWrite(f, randData)
		}
		wg.Done()
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

	if err := deadlineInAbout(dc, DefaultChannelTimeout); err != nil {
		t.Error(err)
	}
	if err := deadlineInAbout(ac, DefaultChannelTimeout); err != nil {
		t.Error(err)
	}

	df, af := oneFlow(t, ctx, dc, aflows, 0)
	if err := deadlineInAbout(dc, DefaultChannelTimeout); err != nil {
		t.Error(err)
	}
	if err := deadlineInAbout(ac, DefaultChannelTimeout); err != nil {
		t.Error(err)
	}
	df.Close()
	af.Close()

	df, af = oneFlow(t, ctx, dc, aflows, 10*time.Minute)
	if err := deadlineInAbout(dc, 10*time.Minute); err != nil {
		t.Error(err)
	}
	if err := deadlineInAbout(ac, DefaultChannelTimeout); err != nil {
		t.Error(err)
	}
	df.Close()
	af.Close()

	// Here the min timeout will come into play.
	df2, af2 := oneFlow(t, ctx, dc, aflows, time.Second)
	if err := deadlineInAbout(dc, time.Minute); err != nil {
		t.Error(err)
	}
	if err := deadlineInAbout(ac, DefaultChannelTimeout); err != nil {
		t.Error(err)
	}
	df2.Close()
	af2.Close()

	// Setup new conns with a default channel timeout below the min.
	dc, ac, derr, aerr = setupConnsOpts(t, "local", "", ctx, ctx, dflows, aflows, nil, nil, Opts{
		HandshakeTimeout: time.Minute,
		ChannelTimeout:   time.Second,
	})
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
	dc, ac, derr, aerr := setupConnsWithTimeout(t, "local", "", dctx, ctx, dflows, aflows, nil, nil, 1*time.Second, Opts{HandshakeTimeout: 4 * time.Second, ChannelTimeout: time.Second})
	if aerr != nil {
		t.Fatalf("setupConnsWithTimeout: unexpectedly failed: %v, %v", derr, aerr)
	}
	if got, want := verror.ErrorID(derr), verror.ErrCanceled.ID; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	dc.Close(ctx, nil)
	ac.Close(ctx, nil)
}

func TestMTUNegotiation(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()
	network, address := "local", ":0"
	versions := version.Supported

	ridep := naming.Endpoint{Protocol: network, Address: address, RoutingID: naming.FixedRoutingID(191341)}
	ep := naming.Endpoint{Protocol: network, Address: address}
	dch := make(chan *Conn)
	ach := make(chan *Conn)
	derrch := make(chan error)
	aerrch := make(chan error)

	accept := func(mtu uint64, conn flow.Conn) {
		dBlessings, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
		d, _, _, err := NewDialed(ctx, conn, ep, ep, versions, peerAuthorizer{dBlessings, nil}, nil, Opts{MTU: mtu})
		dch <- d
		derrch <- err
	}
	dial := func(mtu uint64, conn flow.Conn) {
		a, err := NewAccepted(ctx, nil, conn, ridep, versions, nil, Opts{MTU: mtu})
		ach <- a
		aerrch <- err
	}

	testConn := func(dmtu, amtu, negotiated uint64) {
		dmrw, amrw := flowtest.Pipe(t, ctx, network, address)

		go dial(dmtu, dmrw)
		go accept(amtu, amrw)

		dconn := <-dch
		aconn := <-ach
		if derr, aerr := <-derrch, <-aerrch; derr != nil || aerr != nil {
			t.Fatalf("dial: %v, accept: %v", derr, aerr)
		}

		if got, want := dconn.mtu, negotiated; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := aconn.mtu, negotiated; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		dconn.Close(ctx, nil)
		aconn.Close(ctx, nil)
	}

	testConn(4096, 8192, 4096)
	testConn(8192, 1024, 1024)

}
