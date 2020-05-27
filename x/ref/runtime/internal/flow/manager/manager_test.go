// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"bufio"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	_ "v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/runtime/internal/flow/conn"
	"v.io/x/ref/runtime/internal/flow/flowtest"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

const leakWaitTime = 250 * time.Millisecond

func TestDirectConnection(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	ctx, cancel := context.WithCancel(ctx)

	am := New(ctx, naming.FixedRoutingID(0x5555), nil, 0, 0, nil)
	if _, err := am.Listen(ctx, "tcp", "127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	dm := New(ctx, naming.FixedRoutingID(0x1111), nil, 0, 0, nil)

	testFlows(t, ctx, dm, am, flowtest.AllowAllPeersAuthorizer{})

	cancel()
	<-am.Closed()
	<-dm.Closed()
	shutdown()
}

func TestDialCachedConn(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	ctx, cancel := context.WithCancel(ctx)

	am := New(ctx, naming.FixedRoutingID(0x5555), nil, 0, 0, nil)
	if _, err := am.Listen(ctx, "tcp", "127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}

	dm := New(ctx, naming.FixedRoutingID(0x1111), nil, 0, 0, nil)
	// At first the cache should be empty.
	if got, want := len(dm.(*manager).cache.conns), 0; got != want {
		t.Fatalf("got cache size %v, want %v", got, want)
	}
	// After dialing a connection the cache should hold one connection.
	testFlows(t, ctx, dm, am, flowtest.AllowAllPeersAuthorizer{})
	if got, want := len(dm.(*manager).cache.conns), 1; got != want {
		t.Fatalf("got cache size %v, want %v", got, want)
	}
	old := dm.(*manager).cache.cache[am.RoutingID()][0]
	// After dialing another connection the cache should still hold one connection
	// because the connections should be reused.
	testFlows(t, ctx, dm, am, flowtest.AllowAllPeersAuthorizer{})
	if got, want := len(dm.(*manager).cache.cache[am.RoutingID()]), 1; got != want {
		t.Errorf("got cache size %v, want %v", got, want)
	}
	if c := dm.(*manager).cache.cache[am.RoutingID()][0]; c != old {
		t.Errorf("got %v want %v", c, old)
	}

	cancel()
	<-am.Closed()
	<-dm.Closed()
	shutdown()
}

func TestBidirectionalListeningEndpoint(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	ctx, cancel := context.WithCancel(ctx)

	am := New(ctx, naming.FixedRoutingID(0x5555), nil, 0, 0, nil)
	if _, err := am.Listen(ctx, "tcp", "127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}

	dm := New(ctx, naming.FixedRoutingID(0x1111), nil, 0, 0, nil)
	testFlows(t, ctx, dm, am, flowtest.AllowAllPeersAuthorizer{})
	// Now am should be able to make a flow to dm even though dm is not listening.
	testFlows(t, ctx, am, dm, flowtest.AllowAllPeersAuthorizer{})

	cancel()
	<-am.Closed()
	<-dm.Closed()
	shutdown()
}

func TestPublicKeyOnlyClientBlessings(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	ctx, cancel := context.WithCancel(ctx)

	am := New(ctx, naming.FixedRoutingID(0x5555), nil, 0, 0, nil)
	if _, err := am.Listen(ctx, "tcp", "127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	nulldm := New(ctx, naming.NullRoutingID, nil, 0, 0, nil)
	_, af := testFlows(t, ctx, nulldm, am, flowtest.AllowAllPeersAuthorizer{})
	// Ensure that the remote blessings of the underlying conn of the accepted blessings
	// only has the public key of the client and no certificates.
	if rBlessings := af.Conn().(*conn.Conn).RemoteBlessings(); len(rBlessings.String()) > 0 || rBlessings.PublicKey() == nil {
		t.Errorf("got %v, want no-cert blessings", rBlessings)
	}
	dm := New(ctx, naming.FixedRoutingID(0x1111), nil, 0, 0, nil)
	_, af = testFlows(t, ctx, dm, am, flowtest.AllowAllPeersAuthorizer{})
	// Ensure that the remote blessings of the underlying conn of the accepted flow are
	// non-zero if we did specify a RoutingID.
	if rBlessings := af.Conn().(*conn.Conn).RemoteBlessings(); len(rBlessings.String()) == 0 {
		t.Errorf("got %v, want full blessings", rBlessings)
	}

	cancel()
	<-am.Closed()
	<-dm.Closed()
	<-nulldm.Closed()
	shutdown()
}

func TestStopListening(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	ctx, cancel := context.WithCancel(ctx)

	am := New(ctx, naming.FixedRoutingID(0x5555), nil, 0, 0, nil)
	if _, err := am.Listen(ctx, "tcp", "127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	dm := New(ctx, naming.FixedRoutingID(0x1111), nil, 0, 0, nil)
	testFlows(t, ctx, dm, am, flowtest.AllowAllPeersAuthorizer{})

	lameEP := am.Status().Endpoints[0]
	am.StopListening(ctx)

	if f, err := dm.Dial(ctx, lameEP, flowtest.AllowAllPeersAuthorizer{}, 0); err == nil {
		t.Errorf("dialing a lame duck should fail, but didn't %#v.", f)
	}

	cancel()
	<-am.Closed()
	<-dm.Closed()
	shutdown()
}

func testFlows(t testing.TB, ctx *context.T, dm, am flow.Manager, auth flow.PeerAuthorizer) (df, af flow.Flow) {
	ep := am.Status().Endpoints[0]
	var err error
	df, err = dm.Dial(ctx, ep, auth, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "do you read me?"
	writeLine(df, want) //nolint:errcheck
	af, err = am.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}

	got, err := readLine(af)
	if err != nil {
		t.Error(err)
	}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	want = "i read you"
	if err := writeLine(af, want); err != nil {
		t.Error(err)
	}
	got, err = readLine(df)
	if err != nil {
		t.Error(err)
	}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	return
}

func readLine(f flow.Flow) (string, error) {
	s, err := bufio.NewReader(f).ReadString('\n')
	return strings.TrimRight(s, "\n"), err
}

func writeLine(f flow.Flow, data string) error {
	data += "\n"
	_, err := f.Write([]byte(data))
	return err
}

func BenchmarkDialCachedConn(b *testing.B) {
	defer goroutines.NoLeaks(b, leakWaitTime)()
	ctx, shutdown := test.V23Init()

	am := New(ctx, naming.FixedRoutingID(0x5555), nil, 0, 0, nil)
	if _, err := am.Listen(ctx, "tcp", "127.0.0.1:0"); err != nil {
		b.Fatal(err)
	}
	dm := New(ctx, naming.FixedRoutingID(0x1111), nil, 0, 0, nil)
	// At first the cache should be empty.
	if got, want := len(dm.(*manager).cache.conns), 0; got != want {
		b.Fatalf("got cache size %v, want %v", got, want)
	}
	// After dialing a connection the cache should hold one connection.
	auth := flowtest.AllowAllPeersAuthorizer{}
	testFlows(b, ctx, dm, am, auth)
	if got, want := len(dm.(*manager).cache.conns), 1; got != want {
		b.Fatalf("got cache size %v, want %v", got, want)
	}
	ep := am.Status().Endpoints[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := dm.Dial(ctx, ep, auth, 0); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	shutdown()
	<-am.Closed()
	<-dm.Closed()
}
