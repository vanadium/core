// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ping_test

import (
	"reflect"
	"runtime/debug"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/services/syncbase/ping"
	"v.io/x/ref/test"
)

////////////////////////////////////////////////////////////////////////////////
// Generic helpers

func fatalf(t *testing.T, format string, args ...interface{}) {
	debug.PrintStack()
	t.Fatalf(format, args...)
}

func eq(t *testing.T, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		fatalf(t, "got %v, want %v", got, want)
	}
}

////////////////////////////////////////////////////////////////////////////////
// TestPingInParallel

func ms(d time.Duration) time.Duration {
	return d * time.Millisecond
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		d = -d
	}
	return d
}

var errCanceled = verror.NewErrCanceled(nil)

// mockConnectionClient allows configuration of the return values of Connection.
// Other functions are not used by the ping package, so they aren't properly mocked.
type mockConnectionClient struct {
	ctx *context.T
	// nameToRTT specifies what RTT to return for ManagedConns returned from Connection,
	// keyed by name.
	nameToRTT map[string]time.Duration
}

// PinConnection returns a PinnedConn with RTT as specified in nameToRTT.
// If no entry exists in the map, Connection will only return when ctx is cancelled.
func (c *mockConnectionClient) PinConnection(ctx *context.T, name string, opts ...rpc.CallOpt) (flow.PinnedConn, error) {
	if rtt, ok := c.nameToRTT[name]; ok {
		return newPinnedConn(c.ctx, rtt), nil
	}
	// We simulate cancellation behavior so that our unit tests can avoid being time
	// dependent and run quicker.
	<-ctx.Done()
	return nil, errCanceled
}

func (c *mockConnectionClient) StartCall(_ *context.T, _, _ string, _ []interface{}, _ ...rpc.CallOpt) (rpc.ClientCall, error) {
	panic("StartCall is currently not used by the ping package.")
}
func (c *mockConnectionClient) Call(_ *context.T, _, _ string, _, _ []interface{}, _ ...rpc.CallOpt) error {
	panic("Call is currently not used by the ping package.")
}
func (c *mockConnectionClient) Close()                  {}
func (c *mockConnectionClient) Closed() <-chan struct{} { return c.ctx.Done() }

func newPinnedConn(ctx *context.T, rtt time.Duration) flow.PinnedConn {
	return &mockPinnedConn{&mockConn{ctx: ctx, rtt: rtt}}
}

type mockPinnedConn struct {
	conn *mockConn
}

func (c *mockPinnedConn) Conn() flow.ManagedConn {
	return c.conn
}
func (*mockPinnedConn) Unpin() {}

// mockConn mocks the RTT function of flow.ManagedConn. If PingInParallel starts
// using more fields of flow.ManagedConn, we should mock their implementations here.
type mockConn struct {
	rtt time.Duration
	ctx *context.T
}

func (c *mockConn) RTT() time.Duration { return c.rtt }

func (*mockConn) LocalEndpoint() naming.Endpoint {
	panic("LocalEndpoint is currently not used by the ping package.")
}
func (*mockConn) RemoteEndpoint() naming.Endpoint {
	panic("RemoteEndpoint is currently not used by the ping package.")
}
func (*mockConn) RemoteBlessings() security.Blessings {
	panic("RemoteBlessings is currently not used by the ping package.")
}
func (*mockConn) LocalBlessings() security.Blessings {
	panic("LocalBlessings is currently not used by the ping package.")
}
func (*mockConn) RemoteDischarges() map[string]security.Discharge {
	panic("RemoteDischarges is currently not used by the ping package.")
}
func (*mockConn) LocalDischarges() map[string]security.Discharge {
	panic("LocalDischarges is currently not used by the ping package.")
}
func (*mockConn) CommonVersion() version.RPCVersion {
	panic("CommonVersion is currently not used by the ping package.")
}
func (*mockConn) LastUsed() time.Time {
	panic("LastUsed is currently not used by the ping package.")
}
func (c *mockConn) Closed() <-chan struct{} { return c.ctx.Done() }

// pingResultsEq checks that the given ping results are equal, with a bit of
// slack around expected durations. Errors are considered equal if their verror
// IDs are equal.
func pingResultsEq(t *testing.T, got, want map[string]ping.PingResult) {
	if len(got) != len(want) {
		eq(t, got, want)
	}
	for name, gotRes := range got {
		wantRes, ok := want[name]
		if !ok {
			eq(t, got, want)
		}
		eq(t, gotRes.Name, wantRes.Name)
		eq(t, verror.ErrorID(gotRes.Err), verror.ErrorID(wantRes.Err))
		if gotRes.Err == nil && wantRes.Err == nil {
			eq(t, gotRes.Conn.Conn().RTT(), wantRes.Conn.Conn().RTT())
		}
	}
}

func setupMockClient(ctx *context.T, nameToRTT map[string]time.Duration) *context.T {
	ctx = fake.SetClientFactory(ctx, func(ctx *context.T, opts ...rpc.ClientOpt) rpc.Client {
		return &mockConnectionClient{ctx: ctx, nameToRTT: nameToRTT}
	})
	ctx, _, _ = v23.WithNewClient(ctx)
	return ctx
}

func TestPingInParallel(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	a, b, c := "a", "b", "c"
	// For these tests, we mock the client, so the timeout we pass to PingInParallel
	// doesn't matter. The mock client instead guarantees that we get some results
	// but not others.
	// Since there are entries for all peers in the map, all pings should succeed.
	ctx = setupMockClient(ctx, map[string]time.Duration{a: ms(10), b: ms(50), c: ms(90)})
	res, err := ping.PingInParallel(ctx, []string{a, b, c}, ms(1), 0)
	eq(t, err, nil)
	pingResultsEq(t, res, map[string]ping.PingResult{
		a: {Name: a, Conn: newPinnedConn(ctx, ms(10))},
		b: {Name: b, Conn: newPinnedConn(ctx, ms(50))},
		c: {Name: c, Conn: newPinnedConn(ctx, ms(90))},
	})

	// Since c is not in the map, it will appear as cancelled, as opposed to not appearing.
	ctx = setupMockClient(ctx, map[string]time.Duration{a: ms(10), b: ms(50)})
	res, err = ping.PingInParallel(ctx, []string{a, b, c}, ms(1), 0)
	eq(t, err, nil)
	pingResultsEq(t, res, map[string]ping.PingResult{
		a: {Name: a, Conn: newPinnedConn(ctx, ms(10))},
		b: {Name: b, Conn: newPinnedConn(ctx, ms(50))},
		c: {Name: c, Err: errCanceled},
	})

	// Same as above, but with names in a different order to demonstrate that the
	// order doesn't matter.
	ctx = setupMockClient(ctx, map[string]time.Duration{a: ms(10), b: ms(50)})
	res, err = ping.PingInParallel(ctx, []string{c, b, a}, ms(1), 0)
	eq(t, err, nil)
	pingResultsEq(t, res, map[string]ping.PingResult{
		a: {Name: a, Conn: newPinnedConn(ctx, ms(10))},
		b: {Name: b, Conn: newPinnedConn(ctx, ms(50))},
		c: {Name: c, Err: errCanceled},
	})

	// No servers to ping.
	ctx = setupMockClient(ctx, map[string]time.Duration{})
	res, err = ping.PingInParallel(ctx, []string{}, ms(1), 0)
	eq(t, err, nil)
	pingResultsEq(t, res, map[string]ping.PingResult{})
}
