// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
	connpackage "v.io/x/ref/runtime/internal/flow/conn"
	"v.io/x/ref/runtime/internal/flow/flowtest"
	_ "v.io/x/ref/runtime/protocols/local"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

func init() {
	flow.RegisterProtocol("resolve", &resolveProtocol{
		protocol:  "local",
		addresses: []string{"address"},
	})
	p, _ := flow.RegisteredProtocol("local")
	flow.RegisterProtocol("wrong", p)
}

var nextrid uint64 = 100

func makeEPs(ctx *context.T, addr string) (ep, nullep, wprotoep, waddrep, waddrprotoep, resolvep naming.Endpoint) {
	ep = naming.Endpoint{
		Protocol:  "local",
		Address:   addr,
		RoutingID: naming.FixedRoutingID(nextrid),
	}.WithBlessingNames(unionBlessing(ctx, "A", "B", "C"))
	nextrid++

	nullep = ep
	nullep.RoutingID = naming.NullRoutingID
	wprotoep = nullep
	wprotoep.Protocol = "wrong"
	waddrep = nullep
	waddrep.Address = "wrong"
	resolvep = nullep
	resolvep.Protocol = "resolve"
	resolvep.Address = "wrong"
	waddrprotoep = ep
	waddrprotoep.Protocol = "wrong"
	waddrprotoep.Address = "wrong"
	return
}

//nolint:deadcode,unused
func modep(ep naming.Endpoint, field string, value interface{}) naming.Endpoint {
	reflect.ValueOf(ep).FieldByName(field).Set(reflect.ValueOf(value))
	return ep
}

func makeEP(ctx *context.T, protocol, address string, rid uint64, blessings ...string) naming.Endpoint {
	routingID := naming.NullRoutingID
	if rid != 0 {
		routingID = naming.FixedRoutingID(rid)
	}
	return naming.Endpoint{
		Protocol:  protocol,
		Address:   address,
		RoutingID: routingID,
	}.WithBlessingNames(blessings)
}

func TestCacheReserve(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()

	c := NewConnCache(0)
	defer c.Close(ctx)
	nullep := makeEP(ctx, "local", "a1", 0, "b1")
	oneep := makeEP(ctx, "local", "a1", 1, "b1")
	twoep := makeEP(ctx, "local", "a1", 2, "b1")
	proxyep := makeEP(ctx, "local", "a1", 3, "b1")

	r1 := c.Reserve(ctx, oneep)
	if r1 == nil {
		t.Error("expected non-nil reservation.")
	}

	// That will have reserved both the address and the path.
	// A second reservation should return nil.
	if nr := c.Reserve(ctx, oneep); nr != nil {
		t.Errorf("got %v, want nil", nr)
	}
	if nr := c.Reserve(ctx, nullep); nr != nil {
		t.Errorf("got %v, want nil", nr)
	}

	// Reserving the proxy now will return a reservation for only
	// the proxies RID, since at this point we don't know the proxies
	// RID.
	pr := c.Reserve(ctx, proxyep)
	if pr == nil {
		t.Errorf("epected non-nil reservation")
	}

	// Reserving a different RID, but the same address will result in a path
	// reservation.
	r2 := c.Reserve(ctx, twoep)
	if r2 == nil {
		t.Errorf("expected non-nil reservation.")
	}

	// Since r1 has reserved the proxy address, it's ProxyConn should return nil.
	if pc := r1.ProxyConn(); pc != nil {
		t.Errorf("got %v, expected nil.", pc)
	}
	proxycaf := makeConnAndFlow(t, ctx, proxyep)
	defer proxycaf.stop(ctx)
	onecaf := makeConnAndFlow(t, ctx, oneep)
	defer onecaf.stop(ctx)
	if err := r1.Unreserve(onecaf.c, proxycaf.c, nil); err != nil {
		t.Fatal(err)
	}

	// Now, asking for the ProxyConn on the proxy reservation should find
	// the proxy, since we just inserted it.
	if pc := pr.ProxyConn().(*connpackage.Conn); pc != proxycaf.c {
		t.Errorf("got %v, expected %v", pc, proxycaf.c)
	}

	// Now that the conn exists, we should not get another reservation.
	if nr := c.Reserve(ctx, oneep); nr != nil {
		t.Errorf("got %v, want nil", nr)
	}
	if nr := c.Reserve(ctx, nullep); nr != nil {
		t.Errorf("got %v, want nil", nr)
	}

	// Note that the context should not have been canceled.
	if err := r1.Context().Err(); err != nil {
		t.Errorf("got %v want nil", err)
	}

	pc := r2.ProxyConn()
	if pc == nil || pc.RemoteEndpoint().RoutingID != proxyep.RoutingID {
		t.Fatalf("got %v, want %v", pc.RemoteEndpoint(), proxyep)
	}
	twocaf := makeConnAndFlow(t, ctx, twoep)
	defer twocaf.stop(ctx)

	if err := r2.Unreserve(twocaf.c, pc, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCacheFind(t *testing.T) { //nolint:gocyclo
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()
	c := NewConnCache(0)

	ep, nullep, wprotoep, waddrep, waddrprotoep, resolvep := makeEPs(ctx, "address")

	auth := flowtest.NewPeerAuthorizer(ep.BlessingNames())
	caf := makeConnAndFlow(t, ctx, ep)
	defer caf.stop(ctx)
	conn := caf.c
	if err := c.Insert(conn, false); err != nil {
		t.Fatal(err)
	}
	// We should be able to find the conn in the cache.
	if got, _, _, err := c.Find(ctx, nullep, auth); err != nil || got != conn {
		t.Errorf("got %v, want %v, err: %v", got, conn, err)
	}
	// Changing the protocol should fail.
	if got, _, _, err := c.Find(ctx, wprotoep, auth); err == nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}
	// Changing the address should fail.
	if got, _, _, err := c.Find(ctx, waddrep, auth); err == nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}
	// Changing the blessingNames should fail.
	if got, _, _, err := c.Find(ctx, ep, flowtest.NewPeerAuthorizer([]string{"wrong"})); err == nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}
	// But finding a set of blessings that has at least one blessings in remote.Blessings should succeed.
	if got, _, _, err := c.Find(ctx, ep, flowtest.NewPeerAuthorizer([]string{"foo", ep.BlessingNames()[0]})); err != nil || got != conn {
		t.Errorf("got %v, want %v, err: %v", got, conn, err)
	}
	// Finding by routing ID should work.
	if got, _, _, err := c.Find(ctx, waddrprotoep, auth); err != nil || got != conn {
		t.Errorf("got %v, want %v, err: %v", got, conn, err)
	}
	// Finding by a valid resolve protocol and address should work.
	if got, _, _, err := c.Find(ctx, resolvep, auth); err != nil || got != conn {
		t.Errorf("got %v, want %v, err: %v", got, conn, err)
	}
	// Caching a proxied connection should not care about endpoint blessings, since the
	// blessings only correspond to the end server.
	proxyep, nullProxyep, _, _, _, _ := makeEPs(ctx, "proxy")
	caf = makeConnAndFlow(t, ctx, proxyep)
	defer caf.stop(ctx)
	proxyConn := caf.c
	if err := c.Insert(proxyConn, true); err != nil {
		t.Fatal(err)
	}
	// Wrong blessingNames should still work
	if got, _, _, err := c.Find(ctx, nullProxyep, flowtest.NewPeerAuthorizer([]string{"wrong"})); err != nil || got != proxyConn {
		t.Errorf("got %v, want %v, err: %v", got, proxyConn, err)
	}

	ridep, nullridep, _, _, waddrprotoridep, _ := makeEPs(ctx, "rid")

	// Caching with InsertWithRoutingID should only cache by RoutingID, not with network/address.
	ridauth := flowtest.NewPeerAuthorizer(ridep.BlessingNames())
	caf = makeConnAndFlow(t, ctx, ridep)
	defer caf.stop(ctx)
	ridConn := caf.c
	if err := c.InsertWithRoutingID(ridConn, false); err != nil {
		t.Fatal(err)
	}
	if got, _, _, err := c.Find(ctx, nullridep, ridauth); err == nil || got != nil {
		t.Errorf("got %v, want <nil>, err: %v", got, err)
	}
	// Finding by routing ID should work.
	if got, _, _, err := c.Find(ctx, waddrprotoridep, ridauth); err != nil || got != ridConn {
		t.Errorf("got %v, want %v, err: %v", got, ridConn, err)
	}

	// Insert a duplicate conn to ensure that replaced conns still get closed.
	caf = makeConnAndFlow(t, ctx, ep)
	defer caf.stop(ctx)
	dupConn := caf.c
	if err := c.Insert(dupConn, false); err != nil {
		t.Fatal(err)
	}

	// Closing the cache should close all the connections in the cache.
	// Ensure that the conns are not closed yet.
	if status := conn.Status(); status == connpackage.Closed {
		t.Fatal("wanted conn to not be closed")
	}
	if status := dupConn.Status(); status == connpackage.Closed {
		t.Fatal("wanted dupConn to not be closed")
	}
	c.Close(ctx)

	<-conn.Closed()
	<-ridConn.Closed()
	<-dupConn.Closed()
	<-proxyConn.Closed()
}

func TestLRU(t *testing.T) { //nolint:gocyclo
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Ensure that the least recently created conns are killed by KillConnections.
	c := NewConnCache(0)
	defer c.Close(ctx)
	conns, stop := nConnAndFlows(t, ctx, 10)
	defer stop()
	for _, conn := range conns {
		if err := c.Insert(conn.c, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.KillConnections(ctx, 3); err != nil {
		t.Fatal(err)
	}
	// conns[3:] should not be closed and still in the cache.
	// conns[:3] should be closed and removed from the cache.
	for _, conn := range conns[3:] {
		if status := conn.c.Status(); status == connpackage.Closed {
			t.Errorf("conn %v should not have been closed", conn)
		}
		if !isInCache(ctx, c, conn.c) {
			t.Errorf("conn %v(%p) should still be in cache:\n%s",
				conn.c.RemoteEndpoint(), conn.c, c)
		}
	}
	for _, conn := range conns[:3] {
		<-conn.c.Closed()
		if isInCache(ctx, c, conn.c) {
			t.Errorf("conn %v should not be in cache", conn)
		}
	}

	// Ensure that writing to conns marks conns as more recently used.
	c = NewConnCache(0)
	defer c.Close(ctx)
	conns, stop = nConnAndFlows(t, ctx, 10)
	defer stop()
	for _, conn := range conns {
		if err := c.Insert(conn.c, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, conn := range conns[:7] {
		conn.write()
	}
	if err := c.KillConnections(ctx, 3); err != nil {
		t.Fatal(err)
	}
	// conns[:7] should not be closed and still in the cache.
	// conns[7:] should be closed and removed from the cache.
	for _, conn := range conns[:7] {
		if status := conn.c.Status(); status == connpackage.Closed {
			t.Errorf("conn %v should not have been closed", conn)
		}
		if !isInCache(ctx, c, conn.c) {
			t.Errorf("conn %v should still be in cache", conn)
		}
	}
	for _, conn := range conns[7:] {
		<-conn.c.Closed()
		if isInCache(ctx, c, conn.c) {
			t.Errorf("conn %v should not be in cache", conn)
		}
	}

	// Ensure that reading from conns marks conns as more recently used.
	c = NewConnCache(0)
	defer c.Close(ctx)
	conns, stop = nConnAndFlows(t, ctx, 10)
	defer stop()
	for _, conn := range conns {
		if err := c.Insert(conn.c, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, conn := range conns[:7] {
		conn.read()
	}
	if err := c.KillConnections(ctx, 3); err != nil {
		t.Fatal(err)
	}
	// conns[:7] should not be closed and still in the cache.
	// conns[7:] should be closed and removed from the cache.
	for _, conn := range conns[:7] {
		if status := conn.c.Status(); status == connpackage.Closed {
			t.Errorf("conn %v should not have been closed", conn)
		}
		if !isInCache(ctx, c, conn.c) {
			t.Errorf("conn %v(%p) should still be in cache:\n%s",
				conn.c.RemoteEndpoint(), conn.c, c)
		}
	}
	for _, conn := range conns[7:] {
		<-conn.c.Closed()
		if isInCache(ctx, c, conn.c) {
			t.Errorf("conn %v should not be in cache", conn)
		}
	}
}

func TestIdleConns(t *testing.T) { //nolint:gocyclo
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Ensure that idle conns are killed by the cache.
	// Set the idle timeout very low to ensure all the connections are closed.
	c := NewConnCache(1)
	defer c.Close(ctx)
	conns, stop := nConnAndFlows(t, ctx, 10)
	defer stop()
	for _, conn := range conns {
		// close the flows so the conns aren't kept alive due to ongoing flows.
		conn.f.Close()
		if err := c.Insert(conn.c, false); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(2 * time.Millisecond)
	if err := c.KillConnections(ctx, 0); err != nil {
		t.Fatal(err)
	}
	// All connections should be lameducked.
	for _, conn := range conns {
		if status := conn.c.Status(); status < connpackage.EnteringLameDuck {
			t.Errorf("conn %v should have been lameducked", conn)
		}
	}
	// We wait for the acknowledgement of the lameduck, then KillConnections should
	// actually close the connections.
	for _, conn := range conns {
		<-conn.c.EnterLameDuck(ctx)
	}
	if err := c.KillConnections(ctx, 0); err != nil {
		t.Fatal(err)
	}
	// All connections should be removed from the cache.
	for _, conn := range conns {
		<-conn.c.Closed()
		if isInCache(ctx, c, conn.c) {
			t.Errorf("conn %v should not be in cache", conn)
		}
	}

	// Set the idle timeout very high to ensure none of the connections are closed.
	c = NewConnCache(time.Hour)
	defer c.Close(ctx)
	conns, stop = nConnAndFlows(t, ctx, 10)
	defer stop()
	for _, conn := range conns {
		// close the flows so the conns aren't kept alive due to ongoing flows.
		conn.f.Close()
		if err := c.Insert(conn.c, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.KillConnections(ctx, 0); err != nil {
		t.Fatal(err)
	}
	for _, conn := range conns {
		if status := conn.c.Status(); status >= connpackage.EnteringLameDuck {
			t.Errorf("conn %v should not have been closed or lameducked", conn)
		}
		if !isInCache(ctx, c, conn.c) {
			t.Errorf("conn %v(%p) should still be in cache:\n%s",
				conn.c.RemoteEndpoint(), conn.c, c)
		}
	}

	// Ensure that a low idle timeout, but live flows on the conns keep the connection alive.
	c = NewConnCache(1)
	defer c.Close(ctx)
	conns, stop = nConnAndFlows(t, ctx, 10)
	defer stop()
	for _, conn := range conns {
		// don't close the flows so that the conns are kept alive.
		if err := c.Insert(conn.c, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.KillConnections(ctx, 0); err != nil {
		t.Fatal(err)
	}
	for _, conn := range conns {
		if status := conn.c.Status(); status >= connpackage.LameDuckAcknowledged {
			t.Errorf("conn %v should not have been closed or lameducked", conn)
		}
		if !isInCache(ctx, c, conn.c) {
			t.Errorf("conn %v(%p) should still be in cache:\n%s",
				conn.c.RemoteEndpoint(), conn.c, c)
		}
	}
}

func TestMultiRTTConns(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()

	c := NewConnCache(0)
	defer c.Close(ctx)
	remote, _, _, _, _, _ := makeEPs(ctx, "normal")
	auth := flowtest.NewPeerAuthorizer(remote.BlessingNames())
	slow, med, fast := 3*time.Millisecond, 2*time.Millisecond, 1*time.Millisecond
	// Add a slow connection into the cache and ensure it is found.
	slowConn := newRTTConn(ctx, remote, slow)
	if err := c.Insert(slowConn, false); err != nil {
		t.Fatal(err)
	}
	if got, _, _, err := c.Find(ctx, remote, auth); err != nil || got != slowConn {
		t.Errorf("got %v, want %v, err: %v", got, slowConn, err)
	}

	// Add a fast connection into the cache and ensure it is found over the slow one.
	fastConn := newRTTConn(ctx, remote, fast)
	if err := c.Insert(fastConn, false); err != nil {
		t.Fatal(err)
	}
	if got, _, _, err := c.Find(ctx, remote, auth); err != nil || got != fastConn {
		t.Errorf("got %v, want %v, err: %v", got, fastConn, err)
	}

	// Add a med connection into the cache and ensure that the fast one is still found.
	medConn := newRTTConn(ctx, remote, med)
	if err := c.Insert(medConn, false); err != nil {
		t.Fatal(err)
	}
	if got, _, _, err := c.Find(ctx, remote, auth); err != nil || got != fastConn {
		t.Errorf("got %v, want %v, err: %v", got, fastConn, err)
	}

	// Kill the fast connection and ensure the med connection is found.
	fastConn.Close(ctx, nil)
	if got, _, _, err := c.Find(ctx, remote, auth); err != nil || got != medConn {
		t.Errorf("got %v, want %v, err: %v", got, medConn, err)
	}
}

func newRTTConn(ctx *context.T, ep naming.Endpoint, rtt time.Duration) *rttConn {
	return &rttConn{
		ctx:    ctx,
		ep:     ep,
		rtt:    rtt,
		closed: make(chan struct{}),
	}
}

type rttConn struct {
	ctx    *context.T
	ep     naming.Endpoint
	rtt    time.Duration
	closed chan struct{}
}

func (c *rttConn) Status() connpackage.Status {
	select {
	case <-c.closed:
		return connpackage.Closed
	default:
		return connpackage.Active
	}
}
func (c *rttConn) IsEncapsulated() bool                  { return false }
func (c *rttConn) IsIdle(*context.T, time.Duration) bool { return false }
func (c *rttConn) EnterLameDuck(*context.T) chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (c *rttConn) RemoteLameDuck() bool                       { return false }
func (c *rttConn) CloseIfIdle(*context.T, time.Duration) bool { return false }
func (c *rttConn) Close(*context.T, error) {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
}
func (c *rttConn) RemoteEndpoint() naming.Endpoint { return c.ep }
func (c *rttConn) LocalEndpoint() naming.Endpoint  { return c.ep }
func (c *rttConn) RemoteBlessings() security.Blessings {
	b, _ := v23.GetPrincipal(c.ctx).BlessingStore().Default()
	return b
}
func (c *rttConn) RemoteDischarges() map[string]security.Discharge { return nil }
func (c *rttConn) RTT() time.Duration                              { return c.rtt }
func (c *rttConn) LastUsed() time.Time                             { return time.Now() }
func (c *rttConn) DebugString() string                             { return "" }

func isInCache(ctx *context.T, c *ConnCache, conn *connpackage.Conn) bool {
	rep := conn.RemoteEndpoint()
	_, _, _, err := c.Find(ctx, rep, flowtest.NewPeerAuthorizer(rep.BlessingNames()))
	return err == nil
}

type connAndFlow struct {
	c *connpackage.Conn
	a *connpackage.Conn
	f flow.Flow
}

func (c connAndFlow) write() {
	_, err := c.f.WriteMsg([]byte{0})
	if err != nil {
		panic(err)
	}
}

func (c connAndFlow) read() {
	_, err := c.f.ReadMsg()
	if err != nil {
		panic(err)
	}
}

func (c connAndFlow) stop(ctx *context.T) {
	c.c.Close(ctx, nil)
	c.a.Close(ctx, nil)
}

func nConnAndFlows(t *testing.T, ctx *context.T, n int) ([]connAndFlow, func()) {
	cfs := make([]connAndFlow, n)
	for i := 0; i < n; i++ {
		cfs[i] = makeConnAndFlow(t, ctx, naming.Endpoint{
			Protocol:  "local",
			Address:   strconv.Itoa(i),
			RoutingID: naming.FixedRoutingID(uint64(i + 1)), // We need to have a nonzero rid for bidi.
		})
	}
	return cfs, func() {
		for _, conn := range cfs {
			conn.stop(ctx)
		}
	}
}

func makeConnAndFlow(t *testing.T, ctx *context.T, ep naming.Endpoint) connAndFlow {
	dmrw, amrw := flowtest.Pipe(t, ctx, "local", "")
	dch := make(chan *connpackage.Conn)
	ach := make(chan *connpackage.Conn)
	errCh := make(chan error, 2)
	go func() {
		d, _, _, err := connpackage.NewDialed(ctx, dmrw, ep, ep,
			version.RPCVersionRange{Min: 1, Max: 5},
			flowtest.AllowAllPeersAuthorizer{},
			false,
			time.Minute, 0, nil)
		if err != nil {
			err = fmt.Errorf("Unexpected error: %v", err)
		}
		errCh <- err
		dch <- d
	}()
	fh := fh{t, make(chan struct{})}
	go func() {
		a, err := connpackage.NewAccepted(ctx, nil, amrw, ep,
			version.RPCVersionRange{Min: 1, Max: 5}, time.Minute, 0, fh)
		if err != nil {
			err = fmt.Errorf("Unexpected error: %v", err)
		}
		errCh <- err
		ach <- a
	}()
	conn := <-dch
	aconn := <-ach
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	f, err := conn.Dial(ctx, conn.LocalBlessings(), nil, conn.RemoteEndpoint(), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	// Write a byte to send the openFlow message.
	if _, err := f.Write([]byte{0}); err != nil {
		t.Fatal(err)
	}
	<-fh.ch
	return connAndFlow{conn, aconn, f}
}

type fh struct {
	t  *testing.T
	ch chan struct{}
}

func (h fh) HandleFlow(f flow.Flow) error {
	go func() {
		if _, err := f.WriteMsg([]byte{0}); err != nil {
			h.t.Errorf("failed to write: %v", err)
		}
		close(h.ch)
	}()
	return nil
}

func unionBlessing(ctx *context.T, names ...string) []string {
	principal := v23.GetPrincipal(ctx)
	blessings := make([]security.Blessings, len(names))
	for i, name := range names {
		var err error
		if blessings[i], err = principal.BlessSelf(name); err != nil {
			panic(err)
		}
	}
	union, err := security.UnionOfBlessings(blessings...)
	if err != nil {
		panic(err)
	}
	if err := security.AddToRoots(principal, union); err != nil {
		panic(err)
	}
	if err := principal.BlessingStore().SetDefault(union); err != nil {
		panic(err)
	}
	return security.BlessingNames(principal, union)
}

// resolveProtocol returns a fixed protocol and addresses for its Resolve function.
type resolveProtocol struct {
	protocol  string
	addresses []string
}

func (p *resolveProtocol) Resolve(_ *context.T, _, _ string) (string, []string, error) {
	return p.protocol, p.addresses, nil
}
func (*resolveProtocol) Dial(_ *context.T, _, _ string, _ time.Duration) (flow.Conn, error) {
	return nil, nil
}
func (*resolveProtocol) Listen(_ *context.T, _, _ string) (flow.Listener, error) {
	return nil, nil
}
