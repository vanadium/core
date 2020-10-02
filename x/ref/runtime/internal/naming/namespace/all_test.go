// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace_test

import (
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/runtime/factories/library"
	inamespace "v.io/x/ref/runtime/internal/naming/namespace"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func init() {
	// Slow down expiration of existing connections in case these
	// tests run slowly to deflake them.
	library.ConnectionExpiryDuration = 60 * time.Minute
}

func resolveWithRetry(ctx *context.T, name string, opts ...naming.NamespaceOpt) *naming.MountEntry {
	ns := v23.GetNamespace(ctx)
	for {
		me, err := ns.ShallowResolve(ctx, name, opts...)
		if err == nil {
			return me
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func createContexts(t *testing.T) (sc, c *context.T, cleanup func()) {
	ctx, shutdown := test.V23Init()
	var (
		err    error
		psc    = testutil.NewPrincipal("sc")
		pc     = testutil.NewPrincipal("c")
		bsc, _ = psc.BlessingStore().Default()
		bc, _  = pc.BlessingStore().Default()
	)
	// Setup the principals so that they recognize each other.
	if err := security.AddToRoots(psc, bc); err != nil {
		t.Fatal(err)
	}
	if err := security.AddToRoots(pc, bsc); err != nil {
		t.Fatal(err)
	}
	if sc, err = v23.WithPrincipal(ctx, psc); err != nil {
		t.Fatal(err)
	}
	if c, err = v23.WithPrincipal(ctx, pc); err != nil {
		t.Fatal(err)
	}
	return sc, c, shutdown
}

func boom(t *testing.T, f string, v ...interface{}) {
	t.Logf(f, v...)
	t.Fatal(string(debug.Stack()))
}

// N squared but who cares, this is a little test.
// Ignores dups.
func contains(container, contained []string) (string, bool) {
L:
	for _, d := range contained {
		for _, r := range container {
			if r == d {
				continue L
			}
		}
		return d, false
	}
	return "", true
}

func compare(t *testing.T, caller, name string, got, want []string) {
	// Compare ignoring dups.
	a, foundA := contains(got, want)
	b, foundB := contains(want, got)
	if !foundA {
		t.Logf("%s: %q: failed to find %q: got %v, want %v", caller, name, a, got, want)
		boom(t, "%s: %q: failed to find %q: got %v, want %v", caller, name, a, got, want)
	}
	if !foundB {
		t.Logf("%s: %q: failed to find %q: got %v, want %v", caller, name, a, got, want)
		boom(t, "%s: %q: failed to find %q: got %v, want %v", caller, name, b, got, want)
	}
}

func doGlob(t *testing.T, ctx *context.T, ns namespace.T, pattern string, limit int) []string {
	var replies []string

	sctx, done := context.WithTimeout(ctx, 2*time.Minute)
	defer done()
	rc, err := ns.Glob(sctx, pattern)
	if err != nil {
		boom(t, "Glob(%s): %s", pattern, err)
	}
	for s := range rc {
		switch v := s.(type) {
		case *naming.GlobReplyEntry:
			replies = append(replies, v.Value.Name)
			if limit > 0 && len(replies) > limit {
				boom(t, "Glob returns too many results, perhaps not limiting recursion")
			}
		case *naming.GlobReplyError:
			boom(t, "Glob failed at %q: %v", v.Value.Name, v.Value.Error)
		}
	}
	return replies
}

type testServer struct {
	suffix string
}

func (testServer) KnockKnock(*context.T, rpc.ServerCall) (string, error) {
	return "Who's there?", nil
}

// testServer has the following namespace:
// "" -> {level1} -> {level2}
//nolint:golint // API change required.
func (t *testServer) GlobChildren__(_ *context.T, call rpc.GlobChildrenServerCall, m *glob.Element) error {
	switch t.suffix {
	case "":
		if n := "level1"; m.Match(n) {
			return call.SendStream().Send(naming.GlobChildrenReplyName{Value: n})
		}
	case "level1":
		if n := "level2"; m.Match(n) {
			return call.SendStream().Send(naming.GlobChildrenReplyName{Value: n})
		}
	default:
		return nil
	}
	return nil
}

type dispatcher struct{}

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return &testServer{suffix}, security.AllowEveryone(), nil
}

func knockKnock(t *testing.T, ctx *context.T, name string) {
	client := v23.GetClient(ctx)
	var result string
	if err := client.Call(ctx, name, "KnockKnock", nil, []interface{}{&result}); err != nil {
		boom(t, "Call failed: %s", err)
	}
	if result != "Who's there?" {
		boom(t, "Wrong result: %v", result)
	}
}

func doResolveTest(t *testing.T, fname string, f func(*context.T, string, ...naming.NamespaceOpt) (*naming.MountEntry, error), ctx *context.T, name string, want []string, opts ...naming.NamespaceOpt) {
	maxretries := 5
	var lastErr error
	for i := 0; i < maxretries; i++ {
		me, err := f(ctx, name, opts...)
		if err == nil {
			if i > 0 {
				t.Logf("doResolveTest: retried %d times", i)
			}
			compare(t, fname, name, me.Names(), want)
			return
		}
		if err != nil && verror.Action(err).RetryAction() != 0 {
			boom(t, "Failed to %s %s: %s, attempt %d", fname, name, err, i)
		}
		lastErr = err
	}
	boom(t, "Failed to %s %s: %s after %d attempts", fname, name, lastErr, maxretries)
}

func testResolveToMountTable(t *testing.T, ctx *context.T, ns namespace.T, name string, want ...string) {
	doResolveTest(t, "ResolveToMountTable", ns.ResolveToMountTable, ctx, name, want)
}

func testResolve(t *testing.T, ctx *context.T, ns namespace.T, name string, want ...string) {
	doResolveTest(t, "Resolve", ns.ShallowResolve, ctx, name, want)
}

type serverEntry struct {
	mountPoint string
	stop       func()
	endpoint   naming.Endpoint
	name       string
}

func runServer(t *testing.T, ctx *context.T, disp rpc.Dispatcher, mountPoint string) *serverEntry {
	return run(t, ctx, disp, mountPoint, false)
}

func runMT(t *testing.T, ctx *context.T, mountPoint string) *serverEntry {
	mtd, err := mounttablelib.NewMountTableDispatcher(ctx, "", "", "mounttable")
	if err != nil {
		boom(t, "NewMountTableDispatcher returned error: %v", err)
	}
	return run(t, ctx, mtd, mountPoint, true)
}

func run(t *testing.T, ctx *context.T, disp rpc.Dispatcher, mountPoint string, mt bool) *serverEntry {
	ctx, cancel := context.WithCancel(ctx)
	_, s, err := v23.WithNewDispatchingServer(ctx, mountPoint, disp, options.ServesMountTable(mt))
	if err != nil {
		cancel()
		boom(t, "r.NewServer: %s", err)
	}
	eps := s.Status().Endpoints
	t.Logf("server %q -> %s", eps[0].Name(), mountPoint)
	// Wait until the mount point appears in the mount table.
	if len(mountPoint) > 0 {
		resolveWithRetry(ctx, mountPoint)
	}
	stop := func() {
		cancel()
		<-s.Closed()
	}
	return &serverEntry{mountPoint: mountPoint, stop: stop, endpoint: eps[0], name: eps[0].Name()}
}

const (
	mt1MP = "mt1"
	mt2MP = "mt2"
	mt3MP = "mt3"
	mt4MP = "mt4"
	mt5MP = "mt5"
	j1MP  = "joke1"
	j2MP  = "joke2"
	j3MP  = "joke3"

	ttl = 5 * time.Minute
)

// runMountTables creates a root mountable with some mount tables mounted
// in it: mt{1,2,3,4,5}
func runMountTables(t *testing.T, ctx *context.T) (*serverEntry, map[string]*serverEntry) {
	root := runMT(t, ctx, "")
	if err := v23.GetNamespace(ctx).SetRoots(root.name); err != nil {
		t.Fatal(err)
	}
	t.Logf("mountTable %q -> %s", root.mountPoint, root.endpoint)

	mps := make(map[string]*serverEntry)
	for _, mp := range []string{mt1MP, mt2MP, mt3MP, mt4MP, mt5MP} {
		m := runMT(t, ctx, mp)
		t.Logf("mountTable %q -> %s", mp, m.endpoint)
		mps[mp] = m
	}
	return root, mps
}

// createNamespace creates a hierarchy of mounttables and servers
// as follows:
// /mt1, /mt2, /mt3, /mt4, /mt5, /joke1, /joke2, /joke3.
// That is, mt1 is a mount table mounted in the root mount table,
// joke1 is a server mounted in the root mount table.
func createNamespace(t *testing.T, ctx *context.T) (*serverEntry, map[string]*serverEntry, map[string]*serverEntry, func()) {
	root, mts := runMountTables(t, ctx)
	jokes := make(map[string]*serverEntry)
	// Let's run some non-mount table services.
	for _, j := range []string{j1MP, j2MP, j3MP} {
		disp := &dispatcher{}
		jokes[j] = runServer(t, ctx, disp, j)
	}
	return root, mts, jokes, func() {
		for _, s := range jokes {
			s.stop()
		}
		for _, s := range mts {
			s.stop()
		}
		root.stop()
	}
}

// runNestedMountTables creates some nested mount tables in the hierarchy
// created by createNamespace as follows:
// /mt4/foo, /mt4/foo/bar and /mt4/baz where foo, bar and baz are mount tables.
func runNestedMountTables(t *testing.T, ctx *context.T, mts map[string]*serverEntry) {
	ns := v23.GetNamespace(ctx)
	// Set up some nested mounts and verify resolution.
	for _, m := range []string{"mt4/foo", "mt4/foo/bar"} {
		mts[m] = runMT(t, ctx, m)
	}

	// Use a global name for a mount, rather than a relative one.
	// We directly mount baz into the mt4/foo mount table.
	globalMP := naming.JoinAddressName(mts["mt4/foo"].name, "baz")
	mts["baz"] = runMT(t, ctx, "baz")
	sctx, done := context.WithTimeout(ctx, 2*time.Minute)
	defer done()
	if err := ns.Mount(sctx, globalMP, mts["baz"].name, ttl); err != nil {
		boom(t, "Failed to Mount %s: %s", globalMP, err)
	}
}

// TestNamespaceCommon tests common use of the Namespace library
// against a root mount table and some mount tables mounted on it.
func TestNamespaceCommon(t *testing.T) {
	_, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, jokes, stopper := createNamespace(t, c)
	defer stopper()
	ns := v23.GetNamespace(c)

	// All of the initial mounts are served by the root mounttable
	// and hence ResolveToMountTable should return the root mountable
	// as the address portion of the terminal name for those mounttables.
	testResolveToMountTable(t, c, ns, "", root.name)
	for _, m := range []string{mt2MP, mt3MP, mt5MP} {
		rootMT := naming.Join(root.name, m)
		// All of these mount tables are hosted by the root mount table
		testResolveToMountTable(t, c, ns, m, rootMT)

		// The server registered for each mount point is a mount table
		testResolve(t, c, ns, m, mts[m].name)

		// ResolveToMountTable will walk through to the sub MountTables
		mtbar := naming.Join(m, "bar")
		subMT := naming.Join(mts[m].name, "bar")
		testResolveToMountTable(t, c, ns, mtbar, subMT)
	}

	for _, j := range []string{j1MP, j2MP, j3MP} {
		testResolve(t, c, ns, j, jokes[j].name)
	}
}

// TestNamespaceDetails tests more detailed use of the Namespace library.
func TestNamespaceDetails(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, _, stopper := createNamespace(t, sc)
	defer stopper()

	ns := v23.GetNamespace(c)
	if err := ns.SetRoots(root.name); err != nil {
		t.Fatal(err)
	}

	// /mt2 is not an endpoint. Thus, the example below will fail.
	mt3Server := mts[mt3MP].name
	mt2a := "/mt2/a"
	if err := ns.Mount(c, mt2a, mt3Server, ttl); errors.Is(err, naming.ErrNoSuchName) {
		boom(t, "Successfully mounted %s - expected an err %v, not %v", mt2a, naming.ErrNoSuchName, err)
	}

	// Mount using the relative name.
	// This means walk through mt2 if it already exists and mount within
	// the lower level mount table, if the name doesn't exist we'll create
	// a new name for it.
	mt2a = "mt2/a"
	if err := ns.Mount(c, mt2a, mt3Server, ttl); err != nil {
		boom(t, "Failed to Mount %s: %s", mt2a, err)
	}

	mt2mt := naming.Join(mts[mt2MP].name, "a")
	// The mt2/a is served by the mt2 mount table
	testResolveToMountTable(t, c, ns, mt2a, mt2mt)
	// The server for mt2a is mt3server from the second mount above.
	testResolve(t, c, ns, mt2a, mt3Server)

	// Add two more mounts. The // should be stripped off of the
	// second.
	for _, mp := range []struct{ name, server string }{
		{"mt2", mts[mt4MP].name},
		{"mt2//", mts[mt5MP].name},
	} {
		if err := ns.Mount(c, mp.name, mp.server, ttl, naming.ServesMountTable(true)); err != nil {
			boom(t, "Failed to Mount %s: %s", mp.name, err)
		}
	}

	names := []string{naming.JoinAddressName(mts[mt4MP].name, "a"),
		naming.JoinAddressName(mts[mt5MP].name, "a")}
	names = append(names, naming.JoinAddressName(mts[mt2MP].name, "a"))
	// We now have 3 mount tables prepared to serve mt2/a
	testResolveToMountTable(t, c, ns, "mt2/a", names...)
	names = []string{mts[mt4MP].name, mts[mt5MP].name}
	names = append(names, mts[mt2MP].name)
	testResolve(t, c, ns, "mt2", names...)
}

// TestNestedMounts tests some more deeply nested mounts
func TestNestedMounts(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, _, stopper := createNamespace(t, sc)
	runNestedMountTables(t, sc, mts)
	defer stopper()

	ns := v23.GetNamespace(c)
	if err := ns.SetRoots(root.name); err != nil {
		t.Fatal(err)
	}

	// Set up some nested mounts and verify resolution.
	for _, m := range []string{"mt4/foo", "mt4/foo/bar"} {
		testResolve(t, c, ns, m, mts[m].name)
	}

	testResolveToMountTable(t, c, ns, "mt4/foo",
		naming.JoinAddressName(mts[mt4MP].name, "foo"))
	testResolveToMountTable(t, c, ns, "mt4/foo/bar",
		naming.JoinAddressName(mts["mt4/foo"].name, "bar"))
	testResolveToMountTable(t, c, ns, "mt4/foo/baz",
		naming.JoinAddressName(mts["mt4/foo"].name, "baz"))
}

// TestServers tests invoking RPCs on simple servers
func TestServers(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, jokes, stopper := createNamespace(t, sc)
	defer stopper()
	ns := v23.GetNamespace(c)
	if err := ns.SetRoots(root.name); err != nil {
		t.Fatal(err)
	}

	// Let's run some non-mount table services
	for _, j := range []string{j1MP, j2MP, j3MP} {
		testResolve(t, c, ns, j, jokes[j].name)
		knockKnock(t, c, j)
		globalName := naming.JoinAddressName(mts["mt4"].name, j)
		disp := &dispatcher{}
		gj := "g_" + j
		jokes[gj] = runServer(t, c, disp, globalName)
		testResolve(t, c, ns, "mt4/"+j, jokes[gj].name)
		knockKnock(t, c, "mt4/"+j)
		testResolveToMountTable(t, c, ns, "mt4/"+j, globalName)
		testResolveToMountTable(t, c, ns, "mt4/"+j+"/garbage", globalName+"/garbage")
	}
}

// TestGlob tests some glob patterns.
func TestGlob(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, _, stopper := createNamespace(t, sc)
	runNestedMountTables(t, sc, mts)
	defer stopper()
	ns := v23.GetNamespace(c)
	if err := ns.SetRoots(root.name); err != nil {
		t.Fatal(err)
	}

	tln := []string{"baz", "mt1", "mt2", "mt3", "mt4", "mt5", "joke1", "joke2", "joke3"}
	barbaz := []string{"mt4/foo/bar", "mt4/foo/baz"}
	level12 := []string{"joke1/level1", "joke1/level1/level2", "joke2/level1", "joke2/level1/level2", "joke3/level1", "joke3/level1/level2"}
	foo := append([]string{"mt4/foo"}, barbaz...)
	foo = append(foo, level12...)
	// Try various globs.
	globTests := []struct {
		pattern  string
		expected []string
	}{
		{"*", tln},
		{"x", []string{}},
		{"m*", []string{"mt1", "mt2", "mt3", "mt4", "mt5"}},
		{"mt[2,3]", []string{"mt2", "mt3"}},
		{"*z", []string{"baz"}},
		{"joke1/*", []string{"joke1/level1"}},
		{"j?ke1/level1/*", []string{"joke1/level1/level2"}},
		{"joke1/level1/*", []string{"joke1/level1/level2"}},
		{"joke1/level1/level2/...", []string{"joke1/level1/level2"}},
		{"...", append(append(tln, foo...), "")},
		{"*/...", append(tln, foo...)},
		{"*/foo/*", barbaz},
		{"*/*/*z", []string{"mt4/foo/baz"}},
		{"*/f??/*z", []string{"mt4/foo/baz"}},
		{"mt4/foo/baz", []string{"mt4/foo/baz"}},
	}
	for _, test := range globTests {
		out := doGlob(t, c, ns, test.pattern, 0)
		compare(t, "Glob", test.pattern, out, test.expected)
		// Do the same with a full rooted name.
		out = doGlob(t, c, ns, naming.JoinAddressName(root.name, test.pattern), 0)
		var expectedWithRoot []string
		for _, s := range test.expected {
			expectedWithRoot = append(expectedWithRoot, naming.JoinAddressName(root.name, s))
		}
		compare(t, "Glob", test.pattern, out, expectedWithRoot)
	}
}

type GlobbableServer struct {
	callCount int
	mu        sync.Mutex
}

//nolint:golint // API change required.
func (g *GlobbableServer) Glob__(*context.T, rpc.GlobServerCall, *glob.Glob) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.callCount++
	return nil
}

func (g *GlobbableServer) GetAndResetCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	cnt := g.callCount
	g.callCount = 0

	return cnt
}

// TestGlobEarlyStop tests that Glob doesn't query terminal servers with finished patterns.
func TestGlobEarlyStop(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, mts, _, stopper := createNamespace(t, sc)
	runNestedMountTables(t, sc, mts)
	defer stopper()

	globServer := &GlobbableServer{}
	name := naming.JoinAddressName(mts["mt4/foo/bar"].name, "glob")
	runningGlobServer := runServer(t, c, testutil.LeafDispatcher(globServer, nil), name)
	defer runningGlobServer.stop()

	ns := v23.GetNamespace(c)
	if err := ns.SetRoots(root.name); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		pattern       string
		expectedCalls int
		expected      []string
	}{
		{"mt4/foo/bar/glob", 0, []string{"mt4/foo/bar/glob"}},
		{"mt4/foo/bar/glob/...", 1, []string{"mt4/foo/bar/glob"}},
		{"mt4/foo/bar/glob/*", 1, nil},
		{"mt4/foo/bar/***", 0, []string{"mt4/foo/bar", "mt4/foo/bar/glob"}},
		{"mt4/foo/bar/...", 1, []string{"mt4/foo/bar", "mt4/foo/bar/glob"}},
		{"mt4/foo/bar/*", 0, []string{"mt4/foo/bar/glob"}},
		{"mt4/***/bar/***", 0, []string{"mt4/foo/bar", "mt4/foo/bar/glob"}},
		{"mt4/*/bar/***", 0, []string{"mt4/foo/bar", "mt4/foo/bar/glob"}},
	}
	// Test allowing the tests to descend into leaves.
	for _, test := range tests {
		out := doGlob(t, c, ns, test.pattern, 0)
		compare(t, "Glob", test.pattern, out, test.expected)
		if calls := globServer.GetAndResetCount(); calls != test.expectedCalls {
			boom(t, "Wrong number of Glob calls to terminal server got: %d want: %d.", calls, test.expectedCalls)
		}
	}
}

func TestCycles(t *testing.T) {
	sc, c, cleanup := createContexts(t)
	defer cleanup()

	root, _, _, stopper := createNamespace(t, sc)
	defer stopper()
	ns := v23.GetNamespace(c)
	if err := ns.SetRoots(root.name); err != nil {
		t.Fatal(err)
	}

	c1 := runMT(t, c, "c1")
	c2 := runMT(t, c, "c2")
	c3 := runMT(t, c, "c3")
	defer c1.stop()
	defer c2.stop()
	defer c3.stop()

	m := "c1/c2"
	if err := ns.Mount(c, m, c1.name, ttl, naming.ServesMountTable(true)); err != nil {
		boom(t, "Failed to Mount %s: %s", "c1/c2", err)
	}

	m = "c1/c2/c3"
	if err := ns.Mount(c, m, c3.name, ttl, naming.ServesMountTable(true)); err != nil {
		boom(t, "Failed to Mount %s: %s", m, err)
	}

	m = "c1/c3/c4"
	if err := ns.Mount(c, m, c1.name, ttl, naming.ServesMountTable(true)); err != nil {
		boom(t, "Failed to Mount %s: %s", m, err)
	}

	// Since c1 was mounted with the Serve call, it will have both the tcp and ws endpoints.
	testResolve(t, c, ns, "c1", c1.name)
	testResolve(t, c, ns, "c1/c2", c1.name)
	testResolve(t, c, ns, "c1/c3", c3.name)
	testResolve(t, c, ns, "c1/c3/c4", c1.name)
	testResolve(t, c, ns, "c1/c3/c4/c3/c4", c1.name)
	cycle := "c3/c4"
	for i := 0; i < 40; i++ {
		cycle += "/c3/c4"
	}
	if _, err := ns.Resolve(c, "c1/"+cycle); !errors.Is(err, naming.ErrResolutionDepthExceeded) {
		boom(t, "Failed to detect cycle")
	}

	// Perform the glob with a response length limit and dup suppression.  The dup suppression
	// should win.
	r := doGlob(t, c, ns, "c1/...", 1000)
	if len(r) != 6 {
		t.Fatalf("expected 6 replies, got %v", r)
	}
}

// TestGoroutineLeaks tests for leaking goroutines - we have many:-(
func TestGoroutineLeaks(t *testing.T) {
	t.Skip()
	sc, _, cleanup := createContexts(t)
	defer cleanup()

	_, _, _, stopper := createNamespace(t, sc)
	defer func() {
		sc.Infof("%d goroutines:", runtime.NumGoroutine())
	}()
	defer stopper()
	defer func() {
		sc.Infof("%d goroutines:", runtime.NumGoroutine())
	}()
	// panic("this will show up lots of goroutine+channel leaks!!!!")
}

func TestBadRoots(t *testing.T) {
	if _, err := inamespace.New(); err != nil {
		t.Errorf("namespace.New should not have failed with no roots")
	}
	if _, err := inamespace.New("not a rooted name"); err == nil {
		t.Errorf("namespace.New should have failed with an unrooted name")
	}
}

func TestAuthorizationDuringResolve(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23Init()
	defer shutdown()

	var (
		rootMtCtx, _   = v23.WithPrincipal(ctx, testutil.NewPrincipal()) // root mounttable
		mtCtx, _       = v23.WithPrincipal(ctx, testutil.NewPrincipal()) // intermediate mounttable
		serverCtx, _   = v23.WithPrincipal(ctx, testutil.NewPrincipal()) // end server
		clientCtx, _   = v23.WithPrincipal(ctx, testutil.NewPrincipal()) // client process (doing Resolves).
		clientNs       = v23.GetNamespace(clientCtx)
		serverNs       = v23.GetNamespace(serverCtx)
		idp            = testutil.NewIDProvider("idp") // identity provider
		serverEndpoint = naming.FormatEndpoint("tcp", "127.0.0.1:14141")
	)

	// Setup default blessings for the processes.
	if err := idp.Bless(v23.GetPrincipal(rootMtCtx), "rootmt"); err != nil {
		t.Fatal(err)
	}
	if err := idp.Bless(v23.GetPrincipal(serverCtx), "server"); err != nil {
		t.Fatal(err)
	}
	if err := idp.Bless(v23.GetPrincipal(mtCtx), "childmt"); err != nil {
		t.Fatal(err)
	}
	if err := idp.Bless(v23.GetPrincipal(clientCtx), "client"); err != nil {
		t.Fatal(err)
	}

	// Setup the namespace root for all the "processes".
	rootmt := runMT(t, rootMtCtx, "")
	defer rootmt.stop()

	for _, ctx := range []*context.T{mtCtx, serverCtx, clientCtx} {
		if err := v23.GetNamespace(ctx).SetRoots(rootmt.name); err != nil {
			t.Fatal(err)
		}
	}
	// Disable caching in the client so that any Mount calls by the server
	// are noticed immediately.
	clientNs.CacheCtl(naming.DisableCache(true))

	// Intermediate mounttables should be authenticated.
	mt := runMT(t, mtCtx, "mt")
	defer mt.stop()

	// Mount a server on "mt".
	if err := serverNs.Mount(serverCtx, "mt/server", serverEndpoint, time.Minute, naming.ReplaceMount(true)); err != nil {
		t.Error(err)
	}

	// The namespace root should be authenticated too
	resolveWithRetry(clientCtx, "mt/server")
	// Host:Port and Endpoint versions of the other namespace root
	// (which has different blessings)
	hproot := fmt.Sprintf("(otherroot)@%v", rootmt.endpoint.Addr())
	eproot := naming.FormatEndpoint(rootmt.endpoint.Addr().Network(), rootmt.endpoint.Addr().String(), rootmt.endpoint.RoutingID, naming.BlessingOpt("otherroot"), naming.ServesMountTable(rootmt.endpoint.ServesMountTable))
	ns := v23.GetNamespace(ctx)
	for _, root := range []string{hproot, eproot} {
		name := naming.JoinAddressName(root, "mt")
		// Rooted name resolutions should fail authorization because of the "otherroot"
		if e, err := ns.ShallowResolve(clientCtx, name); !errors.Is(err, verror.ErrNotTrusted) {
			t.Errorf("resolve(%q) returned (%v, errorid=%v %v), wanted errorid=%v", name, e, verror.ErrorID(err), err, verror.ErrNotTrusted.ID)
		}
		// But not fail if the server authorization is skipped.
		if e, err := ns.ShallowResolve(clientCtx, name, options.NameResolutionAuthorizer{Authorizer: security.AllowEveryone()}); err != nil {
			t.Errorf("resolve(%q): Got (%v, %v), expected resolution to succeed", name, e, err)
		}
		// The namespace root from the context should be authorized as well.
		ctx, ns, _ := v23.WithNewNamespace(clientCtx, naming.JoinAddressName(root, ""))
		if e, err := ns.Resolve(ctx, "mt/server"); !errors.Is(err, verror.ErrNotTrusted) {
			t.Errorf("resolve with root=%q returned (%v, errorid=%v %v), wanted errorid=%v: %s", root, e, verror.ErrorID(err), err, verror.ErrNotTrusted.ID, verror.DebugString(err))
		}
		if _, err := ns.Resolve(ctx, "mt/server", options.NameResolutionAuthorizer{Authorizer: security.AllowEveryone()}); err != nil {
			t.Errorf("resolve with root=%q should have succeeded when authorization checks are skipped. Got %v: %s", root, err, verror.DebugString(err))
		}
	}

	// Imagine that the network address of "mt" has been taken over by an
	// attacker. However, this attacker cannot mess with the mount entry
	// for "mt". This would result in "mt" and its mount entry (in the
	// global mounttable) having inconsistent blessings. Simulate this by
	// explicitly changing the mount entry for "mt".
	goodChildMTEndpoint := naming.FormatEndpoint(mt.endpoint.Addr().Network(), mt.endpoint.Addr().String(), naming.BlessingOpt("idp/goodchildmt"), mt.endpoint.RoutingID)
	if err := v23.GetNamespace(mtCtx).Mount(mtCtx, "mt", goodChildMTEndpoint, time.Minute, naming.ServesMountTable(true), naming.ReplaceMount(true)); err != nil {
		t.Error(err)
	}

	if e, err := clientNs.Resolve(serverCtx, "mt/server", options.NameResolutionAuthorizer{Authorizer: security.AllowEveryone()}); err != nil {
		t.Errorf("Resolve should succeed when skipping server authorization. Got (%v, %v) %s", e, err, verror.DebugString(err))
	} else if e, err := clientNs.Resolve(serverCtx, "mt/server"); !errors.Is(err, verror.ErrNotTrusted) {
		t.Errorf("Resolve should have failed with %q because an attacker has taken over the intermediate mounttable. Got\n%+v\nerrorid=%q\nerror=%s", verror.ErrNotTrusted.ID, e, verror.ErrorID(err), verror.DebugString(err))
	}
}

// TestDelete tests deleting some parts of the name space.
func TestDelete(t *testing.T) {
	_, c, cleanup := createContexts(t)
	defer cleanup()
	ns := v23.GetNamespace(c)

	// Create a root mount table with mount tables mounted at mt1, mt1, ...
	root, _, _, stopper := createNamespace(t, c)
	defer stopper()
	if err := ns.SetRoots(root.name); err != nil {
		t.Fatal(err)
	}

	// We should be able to remove servers below the root.
	if err := ns.Delete(c, "mt1", false); err != nil {
		t.Errorf("Delete failed: %s", err)
	}

	// Create a server below one level down.
	if err := ns.Mount(c, "mt2/b/c", "/madeup:1111/server", time.Minute); err != nil {
		t.Errorf("Mount mt2/b/c failed: %s", err)
	}

	// We should not be able to delete mt2/b...
	if err := ns.Delete(c, "mt2/b", false); err == nil {
		t.Errorf("Delete mt2/b should have failed")
	}

	// ...unless we include its children.
	if err := ns.Delete(c, "mt2/b", true); err != nil {
		t.Errorf("Delete failed: %s", err)
	}
}

type leafObject struct{}

func (leafObject) Foo(*context.T, rpc.ServerCall) error {
	return nil
}

func TestLeaf(t *testing.T) {
	_, ctx, cleanup := createContexts(t)
	defer cleanup()
	root := runMT(t, ctx, "")
	defer func() { root.stop() }()

	ns := v23.GetNamespace(ctx)
	if err := ns.SetRoots(root.name); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewServer(ctx, "leaf", &leafObject{}, nil)
	if err != nil {
		boom(t, "v23.WithNewServer: %s", err)
	}
	defer func() {
		cancel()
		<-server.Closed()
	}()

	mountEntry := resolveWithRetry(ctx, "leaf")
	if expected := true; mountEntry.IsLeaf != expected {
		boom(t, "unexpected mountEntry.IsLeaf value. Got %v, expected %v", mountEntry.IsLeaf, expected)
	}

	c, err := ns.Glob(ctx, "leaf")
	if err != nil {
		boom(t, "ns.Glob failed: %v", err)
	}
	count := 0
	for result := range c {
		if me, ok := result.(*naming.GlobReplyEntry); ok {
			count++
			if expected := true; me.Value.IsLeaf != expected {
				boom(t, "unexpected me.IsLeaf value. Got %v, expected %v", me.Value.IsLeaf, expected)
			}
		}
	}
	if count == 0 {
		boom(t, "Glob did not return any results. Expected 1")
	}
}
