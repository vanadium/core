// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime/debug"
	"sort"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/debug/debuglib"
	"v.io/x/ref/services/mounttable/btmtd/internal"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/timekeeper"
)

// Simulate different processes with different runtimes.
// rootCtx is the one running the mounttable service.
const ttlSecs = 60 * 60

func boom(t *testing.T, f string, v ...interface{}) {
	t.Logf(f, v...)
	t.Fatal(string(debug.Stack()))
}

func doMount(t *testing.T, ctx *context.T, ep, suffix, service string, shouldSucceed bool) {
	name := naming.JoinAddressName(ep, suffix)
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, name, "Mount", []interface{}{service, uint32(ttlSecs), 0}, nil, options.Preresolved{}); err != nil {
		if !shouldSucceed {
			return
		}
		boom(t, "Failed to Mount %s onto %s: %s", service, name, err)
	}
}

func doUnmount(t *testing.T, ctx *context.T, ep, suffix, service string, shouldSucceed bool) {
	name := naming.JoinAddressName(ep, suffix)
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, name, "Unmount", []interface{}{service}, nil, options.Preresolved{}); err != nil {
		if !shouldSucceed {
			return
		}
		boom(t, "Failed to Unmount %s off of %s: %s", service, name, err)
	}
}

func doGetPermissions(t *testing.T, ctx *context.T, ep, suffix string, shouldSucceed bool) (perms access.Permissions, version string) {
	name := naming.JoinAddressName(ep, suffix)
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, name, "GetPermissions", nil, []interface{}{&perms, &version}, options.Preresolved{}); err != nil {
		if !shouldSucceed {
			return
		}
		boom(t, "Failed to GetPermissions %s: %s", name, err)
	}
	return
}

func doSetPermissions(t *testing.T, ctx *context.T, ep, suffix string, perms access.Permissions, version string, shouldSucceed bool) {
	name := naming.JoinAddressName(ep, suffix)
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, name, "SetPermissions", []interface{}{perms, version}, nil, options.Preresolved{}); err != nil {
		if !shouldSucceed {
			return
		}
		boom(t, "Failed to SetPermissions %s: %s", name, err)
	}
}

func doDeleteNode(t *testing.T, ctx *context.T, ep, suffix string, shouldSucceed bool) {
	name := naming.JoinAddressName(ep, suffix)
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, name, "Delete", []interface{}{false}, nil, options.Preresolved{}); err != nil {
		if !shouldSucceed {
			return
		}
		boom(t, "Failed to Delete node %s: %s", name, err)
	}
}

func doDeleteSubtree(t *testing.T, ctx *context.T, ep, suffix string, shouldSucceed bool) {
	name := naming.JoinAddressName(ep, suffix)
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, name, "Delete", []interface{}{true}, nil, options.Preresolved{}); err != nil {
		if !shouldSucceed {
			return
		}
		boom(t, "Failed to Delete subtree %s: %s", name, err)
	}
}

func mountentry2names(e *naming.MountEntry) []string {
	names := make([]string, len(e.Servers))
	for idx, s := range e.Servers {
		names[idx] = naming.JoinAddressName(s.Server, e.Name)
	}
	return names
}

func resolve(ctx *context.T, name string) (*naming.MountEntry, error) {
	// Resolve the name one level.
	var entry naming.MountEntry
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, name, "ResolveStep", nil, []interface{}{&entry}, options.Preresolved{}); err != nil {
		return nil, err
	}
	if len(entry.Servers) < 1 {
		return nil, errors.New("resolve returned no servers")
	}
	return &entry, nil
}

func export(t *testing.T, ctx *context.T, name, contents string) {
	// Resolve the name.
	resolved, err := resolve(ctx, name)
	if err != nil {
		boom(t, "Failed to Export.Resolve %s: %s", name, err)
	}
	// Export the value.
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, mountentry2names(resolved)[0], "Export", []interface{}{contents, true}, nil, options.Preresolved{resolved}); err != nil {
		boom(t, "Failed to Export.Call %s to %s: %s", name, contents, err)
	}
}

func checkContents(t *testing.T, ctx *context.T, name, expected string, shouldSucceed bool) {
	// Resolve the name.
	resolved, err := resolve(ctx, name)
	if err != nil {
		if !shouldSucceed {
			return
		}
		boom(t, "Failed to Resolve %s: %s", name, err)
	}
	// Look up the value.
	client := v23.GetClient(ctx)
	call, err := client.StartCall(ctx, mountentry2names(resolved)[0], "Lookup", nil, options.Preresolved{resolved})
	if err != nil {
		if shouldSucceed {
			boom(t, "Failed Lookup.StartCall %s: %s", name, err)
		}
		return
	}
	var contents []byte
	if err := call.Finish(&contents); err != nil {
		if shouldSucceed {
			boom(t, "Failed to Lookup %s: %s", name, err)
		}
		return
	}
	if string(contents) != expected {
		boom(t, "Lookup %s, expected %q, got %q", name, expected, contents)
	}
	if !shouldSucceed {
		boom(t, "Lookup %s, expected failure, got %q", name, contents)
	}
}

func newMT(t *testing.T, permsFile string, rootCtx *context.T, config *internal.Config) (func(), string, *internal.BigTable, timekeeper.ManualTime) {
	reservedDisp := debuglib.NewDispatcher(nil)
	ctx := v23.WithReservedNameDispatcher(rootCtx, reservedDisp)

	bt, shutdownBT, err := internal.NewTestBigTable("mounttable")
	if err != nil {
		boom(t, "NewTestBigTable: %s", err)
	}
	if err := bt.SetupTable(ctx, permsFile); err != nil {
		boom(t, "bt.SetupTable: %s", err)
	}

	// Set clock slightly in the future to avoid triggering bigtable's GC.
	clock := timekeeper.NewManualTime()
	now := time.Now().UTC().Add(time.Hour)
	for {
		delta := now.Sub(clock.Now())
		if delta < time.Second {
			break
		}
		clock.AdvanceTime(delta)
	}
	// Add mount table service.
	internal.SetClock(clock)
	internal.SetGcGracePeriod(0)
	if config == nil {
		config = &internal.Config{}
	}
	mt := internal.NewDispatcher(bt, config)

	// Start serving on a loopback address.
	ctx, cancel := context.WithCancel(ctx)
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", mt, options.ServesMountTable(true))
	if err != nil {
		boom(t, "r.NewServer: %s", err)
	}

	estr := server.Status().Endpoints[0].String()
	t.Logf("endpoint %s", estr)
	return func() {
		cancel()
		<-server.Closed()
		shutdownBT()
	}, estr, bt, clock
}

func newCollection(t *testing.T, rootCtx *context.T) (func(), string) {
	// Start serving a collection service on a loopback address.  This
	// is just a service we can mount and test against.
	ctx, cancel := context.WithCancel(rootCtx)
	_, server, err := v23.WithNewDispatchingServer(ctx, "collection", newCollectionServer())
	if err != nil {
		boom(t, "r.NewServer: %s", err)
	}
	estr := server.Status().Endpoints[0].String()
	t.Logf("endpoint %s", estr)
	return func() {
		cancel()
		<-server.Closed()
	}, estr
}

func TestMountTable(t *testing.T) {
	rootCtx, aliceCtx, bobCtx, shutdown := initTest()
	defer shutdown()

	stop, mtAddr, _, clock := newMT(t, "testdata/test.perms", rootCtx, nil)
	defer stop()
	stop, collectionAddr := newCollection(t, rootCtx)
	defer stop()

	collectionName := naming.JoinAddressName(collectionAddr, "collection")

	// Mount the collection server into the mount table.
	rootCtx.Infof("Mount the collection server into the mount table.")
	doMount(t, rootCtx, mtAddr, "stuff", collectionName, true)

	// Create a few objects and make sure we can read them.
	rootCtx.Infof("Create a few objects.")
	export(t, rootCtx, naming.JoinAddressName(mtAddr, "stuff/the/rain"), "the rain")
	export(t, rootCtx, naming.JoinAddressName(mtAddr, "stuff/in/spain"), "in spain")
	export(t, rootCtx, naming.JoinAddressName(mtAddr, "stuff/falls"), "falls mainly on the plain")
	rootCtx.Infof("Make sure we can read them.")
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "stuff/the/rain"), "the rain", true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "stuff/in/spain"), "in spain", true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "stuff/falls"), "falls mainly on the plain", true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "/stuff/falls"), "falls mainly on the plain", true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "stuff/nonexistant"), "falls mainly on the plain", false)
	checkContents(t, bobCtx, naming.JoinAddressName(mtAddr, "stuff/the/rain"), "the rain", true)
	checkContents(t, aliceCtx, naming.JoinAddressName(mtAddr, "stuff/the/rain"), "the rain", false)

	// Test name element too long.
	rootCtx.Infof("Name element too long.")
	doMount(t, rootCtx, mtAddr, "a/abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnop", collectionName, false)

	// Test multiple mounts.
	rootCtx.Infof("Multiple mounts.")
	doMount(t, rootCtx, mtAddr, "a/b", collectionName, true)
	doMount(t, rootCtx, mtAddr, "x/y", collectionName, true)
	doMount(t, rootCtx, mtAddr, "alpha//beta", collectionName, true)
	rootCtx.Infof("Make sure we can read them.")
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "stuff/falls"), "falls mainly on the plain", true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "a/b/falls"), "falls mainly on the plain", true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "x/y/falls"), "falls mainly on the plain", true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "alpha/beta/falls"), "falls mainly on the plain", true)
	checkContents(t, aliceCtx, naming.JoinAddressName(mtAddr, "a/b/falls"), "falls mainly on the plain", true)
	checkContents(t, bobCtx, naming.JoinAddressName(mtAddr, "a/b/falls"), "falls mainly on the plain", false)

	// Test getting/setting AccessLists.
	perms, version := doGetPermissions(t, rootCtx, mtAddr, "stuff", true)
	doSetPermissions(t, rootCtx, mtAddr, "stuff", perms, "xyzzy", false) // bad version
	doSetPermissions(t, rootCtx, mtAddr, "stuff", perms, version, true)  // correct version
	_, nversion := doGetPermissions(t, rootCtx, mtAddr, "stuff", true)
	if nversion == version {
		boom(t, "version didn't change after SetPermissions: %s", nversion)
	}
	doSetPermissions(t, rootCtx, mtAddr, "stuff", perms, "", true) // no version

	// Bob should be able to create nodes under the mounttable root but not alice.
	doSetPermissions(t, aliceCtx, mtAddr, "onlybob", perms, "", false)
	doSetPermissions(t, bobCtx, mtAddr, "onlybob", perms, "", true)

	// Test that setting Permissions to permissions that don't include the the setter's
	// blessings in Admin, automatically add their Blessings to Admin to prevent
	// locking everyone out.
	perms, _ = doGetPermissions(t, bobCtx, mtAddr, "onlybob", true)
	noRootPerms := perms.Copy()
	noRootPerms.Clear("bob", "Admin")
	doSetPermissions(t, bobCtx, mtAddr, "onlybob", noRootPerms, "", true)
	// This should succeed, because "bob" should automatically be added to "Admin"
	// even though he cleared himself from "Admin".
	doSetPermissions(t, bobCtx, mtAddr, "onlybob", perms, "", true)
	// Test that adding a non-standard perms is normalized when retrieved.
	admin := perms["Admin"]
	admin.In = []security.BlessingPattern{"bob", "bob"}
	perms["Admin"] = admin
	doSetPermissions(t, bobCtx, mtAddr, "onlybob", perms, "", true)
	perms, _ = doGetPermissions(t, bobCtx, mtAddr, "onlybob", true)
	if got, want := perms["Admin"].In, []security.BlessingPattern{"bob"}; !reflect.DeepEqual(got, want) {
		boom(t, "got %v, want %v", got, want)
	}

	// Test generic unmount.
	rootCtx.Info("Test generic unmount.")
	doUnmount(t, rootCtx, mtAddr, "a/b", "", true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "a/b/falls"), "falls mainly on the plain", false)

	// Test specific unmount.
	rootCtx.Info("Test specific unmount.")
	doMount(t, rootCtx, mtAddr, "a/b", collectionName, true)
	doUnmount(t, rootCtx, mtAddr, "a/b", collectionName, true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "a/b/falls"), "falls mainly on the plain", false)

	// Try timing out a mount.
	rootCtx.Info("Try timing out a mount.")
	doMount(t, rootCtx, mtAddr, "stuffWithTTL", collectionName, true)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "stuffWithTTL/the/rain"), "the rain", true)
	clock.AdvanceTime(time.Duration(ttlSecs+4) * time.Second)
	checkContents(t, rootCtx, naming.JoinAddressName(mtAddr, "stuffWithTTL/the/rain"), "the rain", false)

	// Test unauthorized mount.
	rootCtx.Info("Test unauthorized mount.")
	doMount(t, bobCtx, mtAddr, "/a/b", collectionName, false)
	doMount(t, aliceCtx, mtAddr, "/a/b", collectionName, false)

	doUnmount(t, bobCtx, mtAddr, "x/y", collectionName, false)
}

func doGlobX(t *testing.T, ctx *context.T, ep, suffix, pattern string, joinServer bool) []string {
	name := naming.JoinAddressName(ep, suffix)
	client := v23.GetClient(ctx)
	call, err := client.StartCall(ctx, name, rpc.GlobMethod, []interface{}{pattern}, options.Preresolved{})
	if err != nil {
		boom(t, "Glob.StartCall %s %s: %s", name, pattern, err)
	}
	var reply []string
	for {
		var gr naming.GlobReply
		err := call.Recv(&gr)
		if err == io.EOF {
			break
		}
		if err != nil {
			boom(t, "Glob.StartCall %s: %s", name, pattern, err)
		}
		switch v := gr.(type) {
		case naming.GlobReplyEntry:
			if joinServer && len(v.Value.Servers) > 0 {
				reply = append(reply, naming.JoinAddressName(v.Value.Servers[0].Server, v.Value.Name))
			} else {
				reply = append(reply, v.Value.Name)
			}
		}
	}
	if err := call.Finish(); err != nil {
		boom(t, "Glob.Finish %s: %s", name, pattern, err)
	}
	return reply
}

func doGlob(t *testing.T, ctx *context.T, ep, suffix, pattern string) []string {
	return doGlobX(t, ctx, ep, suffix, pattern, false)
}

// checkMatch verified that the two slices contain the same string items, albeit
// not necessarily in the same order.  Item repetitions are allowed, but their
// numbers need to match as well.
func checkMatch(t *testing.T, want []string, got []string) {
	if len(want) == 0 && len(got) == 0 {
		return
	}
	w := sort.StringSlice(want)
	w.Sort()
	g := sort.StringSlice(got)
	g.Sort()
	if !reflect.DeepEqual(w, g) {
		boom(t, "Glob expected %v got %v", want, got)
	}
}

// checkExists makes sure a name exists (or not).
func checkExists(t *testing.T, ctx *context.T, ep, suffix string, shouldSucceed bool) {
	x := doGlobX(t, ctx, ep, "", suffix, false)
	if len(x) != 1 || x[0] != suffix {
		if shouldSucceed {
			boom(t, "Failed to find %s", suffix)
		}
		return
	}
	if !shouldSucceed {
		boom(t, "%s exists but shouldn't", suffix)
	}
}

func TestGlob(t *testing.T) {
	rootCtx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	stop, estr, _, _ := newMT(t, "", rootCtx, nil)
	defer stop()

	// set up a mount space
	fakeServer := naming.JoinAddressName(estr, "quux")
	doMount(t, rootCtx, estr, "one/bright/day", fakeServer, true)
	doMount(t, rootCtx, estr, "in/the/middle", fakeServer, true)
	doMount(t, rootCtx, estr, "of/the/night", fakeServer, true)

	// Try various globs.
	tests := []struct {
		in       string
		expected []string
	}{
		{"*", []string{"one", "in", "of"}},
		{"...", []string{"", "one", "in", "of", "one/bright", "in/the", "of/the", "one/bright/day", "in/the/middle", "of/the/night"}},
		{"*/...", []string{"one", "in", "of", "one/bright", "in/the", "of/the", "one/bright/day", "in/the/middle", "of/the/night"}},
		{"one/...", []string{"one", "one/bright", "one/bright/day"}},
		{"of/the/night/two/dead/boys", []string{"of/the/night"}},
		{"*/the", []string{"in/the", "of/the"}},
		{"*/the/...", []string{"in/the", "of/the", "in/the/middle", "of/the/night"}},
		{"o*", []string{"one", "of"}},
		{"", []string{""}},
	}
	for _, test := range tests {
		out := doGlob(t, rootCtx, estr, "", test.in)
		checkMatch(t, test.expected, out)
	}

	// Test Glob on a name that is under a mounted server. The result should the
	// the address the mounted server with the extra suffix.
	{
		results := doGlobX(t, rootCtx, estr, "of/the/night/two/dead/boys/got/up/to/fight", "*", true)
		if len(results) != 1 {
			boom(t, "Unexpected number of results. Got %v, want 1", len(results))
		}
		_, suffix := naming.SplitAddressName(results[0])
		if expected := "quux/two/dead/boys/got/up/to/fight"; suffix != expected {
			boom(t, "Unexpected suffix. Got %v, want %v", suffix, expected)
		}
	}
}

type fakeServerCall struct {
	sendCount int
}

func (fakeServerCall) Security() security.Call              { return security.NewCall(&security.CallParams{}) }
func (fakeServerCall) Suffix() string                       { return "" }
func (fakeServerCall) LocalEndpoint() naming.Endpoint       { return naming.Endpoint{} }
func (fakeServerCall) RemoteEndpoint() naming.Endpoint      { return naming.Endpoint{} }
func (fakeServerCall) GrantedBlessings() security.Blessings { return security.Blessings{} }
func (fakeServerCall) Server() rpc.Server                   { return nil }
func (c *fakeServerCall) SendStream() interface {
	Send(naming.GlobReply) error
} {
	return c
}
func (c *fakeServerCall) Send(reply naming.GlobReply) error {
	c.sendCount++
	return nil
}

/*
func TestGlobAborts(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	mt, err := mounttablelib.NewMountTableDispatcher(ctx, "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	mount := func(name string) error {
		invoker, _, _ := mt.Lookup(ctx, name)
		server := naming.FormatEndpoint("tcp", name)
		return invoker.(mounttable.MountTableServerStub).Mount(ctx, fakeServerCall{}, server, 0, 0)
	}
	// Mount 125 entries: 5 "directories" with 25 entries each.
	for i := 0; i < 5; i++ {
		for j := 0; j < 25; j++ {
			if err := mount(fmt.Sprintf("%d/%d", i, j)); err != nil {
				t.Fatalf("%v (%d, %d)", err, i, j)
			}
		}
	}

	glob := func(ctx *context.T) (int, error) {
		root, _, _ := mt.Lookup(ctx, "")
		g, _ := glob.Parse("...")
		fCall := &fakeServerCall{}
		root.(rpc.Globber).Globber().AllGlobber.Glob__(ctx, fCall, g)
		return fCall.sendCount, nil
	}

	got, err := glob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := 5 + 125 + 1; got != want { // 5 "directories", 125 entries, 1 root entry
		t.Errorf("Got %d want %d", got, want)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err = glob(canceled); err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("Glob returned entries even though the context was cancelled first (returned %d)", got)
	}
}
*/

func TestAccessListTemplate(t *testing.T) {
	rootCtx, aliceCtx, bobCtx, shutdown := initTest()
	defer shutdown()

	stop, estr, _, _ := newMT(t, "testdata/test.perms", rootCtx, nil)
	defer stop()
	fakeServer := naming.JoinAddressName(estr, "quux")

	// Noone should be able to mount on someone else's names.
	doMount(t, aliceCtx, estr, "users/ted", fakeServer, false)
	doMount(t, bobCtx, estr, "users/carol", fakeServer, false)
	doMount(t, rootCtx, estr, "users/george", fakeServer, false)

	// Anyone should be able to mount on their own names.
	doMount(t, aliceCtx, estr, "users/alice", fakeServer, true)
	doMount(t, bobCtx, estr, "users/bob", fakeServer, true)
	doMount(t, rootCtx, estr, "users/root", fakeServer, true)

	// Make sure the counter works.
	doUnmount(t, aliceCtx, estr, "users/alice", "", true)
	doUnmount(t, bobCtx, estr, "users/bob", "", true)
	doUnmount(t, rootCtx, estr, "users/root", "", true)
	perms := access.Permissions{"Admin": access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}}}
	doSetPermissions(t, aliceCtx, estr, "users/alice/a/b/c/d", perms, "", true)
	doSetPermissions(t, aliceCtx, estr, "users/alice/a/b/c/d", perms, "", true)

	/*
		// Do we obey limits?
		for i := 0; i < mounttablelib.DefaultMaxNodesPerUser()-5; i++ {
			node := fmt.Sprintf("users/alice/a/b/c/d/%d", i)
			doSetPermissions(t, aliceCtx, estr, node, perms, "", true)
		}
		doSetPermissions(t, aliceCtx, estr, "users/alice/a/b/c/d/straw", perms, "", false)

		// See if the stats numbers are correct.
		testcases := []struct {
			key      string
			expected interface{}
		}{
			{"alice", int64(mounttablelib.DefaultMaxNodesPerUser())},
			{"bob", int64(0)},
			{"root", int64(0)},
			{conventions.ServerUser, int64(3)},
		}
		for _, tc := range testcases {
			name := "testAccessListTemplate/num-nodes-per-user/" + tc.key
			got, err := libstats.Value(name)
			if err != nil {
				t.Errorf("unexpected error getting map entry for %s: %s", name, err)
			}
			if got != tc.expected {
				t.Errorf("unexpected getting map entry for %s. Got %v, want %v", name, got, tc.expected)
			}
		}
	*/
}

func TestGlobAccessLists(t *testing.T) {
	rootCtx, aliceCtx, bobCtx, shutdown := initTest()
	defer shutdown()

	stop, estr, _, _ := newMT(t, "testdata/test.perms", rootCtx, nil)
	defer stop()

	// set up a mount space
	fakeServer := naming.JoinAddressName(estr, "quux")
	doMount(t, aliceCtx, estr, "one/bright/day", fakeServer, false) // Fails because alice can't mount there.
	doMount(t, bobCtx, estr, "one/bright/day", fakeServer, true)
	doMount(t, rootCtx, estr, "a/b/c", fakeServer, true)

	// Try various globs.
	tests := []struct {
		ctx      *context.T
		in       string
		expected []string
	}{
		{rootCtx, "*", []string{"one", "a", "stuff", "users"}},
		{aliceCtx, "*", []string{"one", "a", "users"}},
		{bobCtx, "*", []string{"one", "stuff", "users"}},
		// bob, alice, and root have different visibility to the space.
		{rootCtx, "*/...", []string{"one", "a", "one/bright", "a/b", "one/bright/day", "a/b/c", "stuff", "users"}},
		{aliceCtx, "*/...", []string{"one", "a", "one/bright", "a/b", "one/bright/day", "a/b/c", "users"}},
		{bobCtx, "*/...", []string{"one", "one/bright", "one/bright/day", "stuff", "users"}},
	}
	for _, test := range tests {
		out := doGlob(t, test.ctx, estr, "", test.in)
		checkMatch(t, test.expected, out)
	}
}

func TestCleanup(t *testing.T) {
	rootCtx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	stop, estr, _, _ := newMT(t, "", rootCtx, nil)
	defer stop()

	// Set up one mount.
	fakeServer := naming.JoinAddressName(estr, "quux")
	doMount(t, rootCtx, estr, "one/bright/day", fakeServer, true)
	checkMatch(t, []string{"one", "one/bright", "one/bright/day"}, doGlob(t, rootCtx, estr, "", "*/..."))

	// After the unmount nothing should be left
	doUnmount(t, rootCtx, estr, "one/bright/day", "", true)
	checkMatch(t, nil, doGlob(t, rootCtx, estr, "", "one"))
	checkMatch(t, nil, doGlob(t, rootCtx, estr, "", "*/..."))

	// Set up a mount, then set the AccessList.
	doMount(t, rootCtx, estr, "one/bright/day", fakeServer, true)
	checkMatch(t, []string{"one", "one/bright", "one/bright/day"}, doGlob(t, rootCtx, estr, "", "*/..."))
	perms := access.Permissions{"Read": access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}}}
	doSetPermissions(t, rootCtx, estr, "one/bright", perms, "", true)

	// After the unmount we should still have everything above the AccessList.
	doUnmount(t, rootCtx, estr, "one/bright/day", "", true)
	checkMatch(t, []string{"one", "one/bright"}, doGlob(t, rootCtx, estr, "", "*/..."))
}

func TestDelete(t *testing.T) {
	rootCtx, aliceCtx, bobCtx, shutdown := initTest()
	defer shutdown()

	stop, estr, _, _ := newMT(t, "testdata/test.perms", rootCtx, nil)
	defer stop()

	// set up a mount space
	fakeServer := naming.JoinAddressName(estr, "quux")
	doMount(t, bobCtx, estr, "one/bright/day", fakeServer, true)
	doMount(t, rootCtx, estr, "a/b/c", fakeServer, true)

	// It shouldn't be possible to delete anything with children unless explicitly requested.
	doDeleteNode(t, rootCtx, estr, "a/b", false)
	checkExists(t, rootCtx, estr, "a/b", true)
	doDeleteSubtree(t, rootCtx, estr, "a/b", true)
	checkExists(t, rootCtx, estr, "a/b", false)

	// Alice shouldn't be able to delete what bob created but bob and root should.
	doDeleteNode(t, aliceCtx, estr, "one/bright/day", false)
	checkExists(t, rootCtx, estr, "one/bright/day", true)
	doDeleteNode(t, rootCtx, estr, "one/bright/day", true)
	checkExists(t, rootCtx, estr, "one/bright/day", false)
	doDeleteNode(t, bobCtx, estr, "one/bright", true)
	checkExists(t, rootCtx, estr, "one/bright", false)

	// Make sure directory admin can delete directory children.
	perms := access.Permissions{"Admin": access.AccessList{In: []security.BlessingPattern{"bob"}}}
	doSetPermissions(t, bobCtx, estr, "hoohaa", perms, "", false)
	doDeleteNode(t, rootCtx, estr, "hoohaa", true)
	checkExists(t, rootCtx, estr, "hoohaa", false)
}

func TestServerFormat(t *testing.T) {
	rootCtx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	stop, estr, _, _ := newMT(t, "", rootCtx, nil)
	defer stop()

	doMount(t, rootCtx, estr, "endpoint", naming.JoinAddressName(estr, "life/on/the/mississippi"), true)
	doMount(t, rootCtx, estr, "hostport", "/atrampabroad:8000", true)
	doMount(t, rootCtx, estr, "invalid/not/rooted", "atrampabroad:8000", false)
	doMount(t, rootCtx, estr, "invalid/no/port", "/atrampabroad", false)
	doMount(t, rootCtx, estr, "invalid/endpoint", "/@following the equator:8000@@@", false)
}

func TestExpiry(t *testing.T) {
	rootCtx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	stop, estr, _, clock := newMT(t, "", rootCtx, nil)
	defer stop()
	stop, collectionAddr := newCollection(t, rootCtx)
	defer stop()

	collectionName := naming.JoinAddressName(collectionAddr, "collection")

	doMount(t, rootCtx, estr, "a1/b1", collectionName, true)
	doMount(t, rootCtx, estr, "a1/b2", collectionName, true)
	doMount(t, rootCtx, estr, "a2/b1", collectionName, true)
	doMount(t, rootCtx, estr, "a2/b2/c", collectionName, true)

	checkMatch(t, []string{"a1/b1", "a2/b1"}, doGlob(t, rootCtx, estr, "", "*/b1/..."))
	clock.AdvanceTime(time.Duration(ttlSecs/2) * time.Second)
	checkMatch(t, []string{"a1/b1", "a2/b1"}, doGlob(t, rootCtx, estr, "", "*/b1/..."))
	checkMatch(t, []string{"c"}, doGlob(t, rootCtx, estr, "a2/b2", "*"))
	// Refresh only a1/b1.  All the other mounts will expire upon the next
	// ft advance.
	doMount(t, rootCtx, estr, "a1/b1", collectionName, true)
	clock.AdvanceTime(time.Duration(ttlSecs/2+4) * time.Second)
	resolve(rootCtx, naming.JoinAddressName(estr, "a2/b2/c"))
	resolve(rootCtx, naming.JoinAddressName(estr, "a2/b1"))
	checkMatch(t, []string{"a1", "a1/b1"}, doGlob(t, rootCtx, estr, "", "*/..."))
	checkMatch(t, []string{"a1/b1"}, doGlob(t, rootCtx, estr, "", "*/b1/..."))
}

func TestNodeCounters(t *testing.T) {
	rootCtx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	stop, estr, bt, clock := newMT(t, "", rootCtx, nil)
	defer stop()

	nodeCount := func() int64 {
		counters, err := bt.Counters(rootCtx)
		if err != nil {
			t.Fatalf("bt.Counters failed: %v", err)
		}
		return counters["num-nodes-per-user:test-blessing"]
	}
	serverCount := func() int64 {
		counters, err := bt.Counters(rootCtx)
		if err != nil {
			t.Fatalf("bt.Counters failed: %v", err)
		}
		return counters["num-servers-per-user:test-blessing"]
	}

	// Test flat tree
	for i := 1; i <= 10; i++ {
		name := fmt.Sprintf("node%d", i)
		addr := naming.JoinAddressName(estr, name)
		doMount(t, rootCtx, estr, name, addr, true)
		if expected, got := int64(i), nodeCount(); got != expected {
			t.Errorf("Unexpected number of nodes. Got %d, expected %d", got, expected)
		}
		if expected, got := int64(i), serverCount(); got != expected {
			t.Errorf("Unexpected number of servers. Got %d, expected %d", got, expected)
		}
	}
	for i := 1; i <= 10; i++ {
		name := fmt.Sprintf("node%d", i)
		if i%2 == 0 {
			doUnmount(t, rootCtx, estr, name, "", true)
		} else {
			doDeleteSubtree(t, rootCtx, estr, name, true)
		}
		if expected, got := int64(10-i), nodeCount(); got != expected {
			t.Errorf("Unexpected number of nodes. Got %d, expected %d", got, expected)
		}
		if expected, got := int64(10-i), serverCount(); got != expected {
			t.Errorf("Unexpected number of server. Got %d, expected %d", got, expected)
		}
	}

	// Test deep tree
	doMount(t, rootCtx, estr, "1/2/3/4/5/6/7/8/9a/10", naming.JoinAddressName(estr, ""), true)
	doMount(t, rootCtx, estr, "1/2/3/4/5/6/7/8/9b/11", naming.JoinAddressName(estr, ""), true)
	if expected, got := int64(12), nodeCount(); got != expected {
		t.Errorf("Unexpected number of nodes. Got %d, expected %d", got, expected)
	}
	if expected, got := int64(2), serverCount(); got != expected {
		t.Errorf("Unexpected number of servers. Got %d, expected %d", got, expected)
	}
	doDeleteSubtree(t, rootCtx, estr, "1/2/3/4/5", true)
	if expected, got := int64(0), nodeCount(); got != expected {
		t.Errorf("Unexpected number of nodes. Got %d, expected %d", got, expected)
	}
	if expected, got := int64(0), serverCount(); got != expected {
		t.Errorf("Unexpected number of servers. Got %d, expected %d", got, expected)
	}

	// Test multiple servers per node
	for i := 1; i <= 5; i++ {
		server := naming.JoinAddressName(estr, fmt.Sprintf("addr%d", i))
		doMount(t, rootCtx, estr, "node1", server, true)
		doMount(t, rootCtx, estr, "node2", server, true)
		if expected, got := int64(2), nodeCount(); got != expected {
			t.Errorf("Unexpected number of nodes. Got %d, expected %d", got, expected)
		}
		if expected, got := int64(2*i), serverCount(); got != expected {
			t.Errorf("Unexpected number of servers. Got %d, expected %d", got, expected)
		}
	}
	doUnmount(t, rootCtx, estr, "node1", "", true)
	if expected, got := int64(1), nodeCount(); got != expected {
		t.Errorf("Unexpected number of nodes. Got %d, expected %d", got, expected)
	}
	if expected, got := int64(5), serverCount(); got != expected {
		t.Errorf("Unexpected number of servers. Got %d, expected %d", got, expected)
	}
	for i := 1; i <= 5; i++ {
		server := naming.JoinAddressName(estr, fmt.Sprintf("addr%d", i))
		doUnmount(t, rootCtx, estr, "node2", server, true)
		expectedNodes := int64(1)
		if i == 5 {
			expectedNodes = 0
		}
		if expected, got := expectedNodes, nodeCount(); got != expected {
			t.Errorf("Unexpected number of nodes. Got %d, expected %d", got, expected)
		}
		if expected, got := int64(5-i), serverCount(); got != expected {
			t.Errorf("Unexpected number of servers. Got %d, expected %d", got, expected)
		}
	}

	// Test expired mounts
	doMount(t, rootCtx, estr, "1/2/3/4/5", naming.JoinAddressName(estr, ""), true)
	if expected, got := int64(5), nodeCount(); got != expected {
		t.Errorf("Unexpected number of nodes. Got %d, expected %d", got, expected)
	}
	if expected, got := int64(1), serverCount(); got != expected {
		t.Errorf("Unexpected number of servers. Got %d, expected %d", got, expected)
	}

	clock.AdvanceTime(time.Duration(ttlSecs+4) * time.Second)
	if _, err := resolve(rootCtx, naming.JoinAddressName(estr, "1/2/3/4/5")); err == nil {
		t.Errorf("Expected failure. Got success")
	}
	if expected, got := int64(0), serverCount(); got != expected {
		t.Errorf("Unexpected number of servers. Got %d, expected %d", got, expected)
	}
}

func TestIntermediateNodesCreatedFromConfig(t *testing.T) {
	rootCtx, _, _, shutdown := initTest()
	defer shutdown()

	stop, estr, _, _ := newMT(t, "testdata/intermediate.perms", rootCtx, nil)
	defer stop()

	// x and x/y should have the same permissions at the root.
	rootPerms, _ := doGetPermissions(t, rootCtx, estr, "", true)
	if perms, _ := doGetPermissions(t, rootCtx, estr, "x", true); !reflect.DeepEqual(rootPerms, perms) {
		boom(t, "for x got %v, want %v", perms, rootPerms)
	}
	if perms, _ := doGetPermissions(t, rootCtx, estr, "x/y", true); !reflect.DeepEqual(rootPerms, perms) {
		boom(t, "for x/y got %v, want %v", perms, rootPerms)
	}
	if perms, _ := doGetPermissions(t, rootCtx, estr, "x/y/z", true); reflect.DeepEqual(rootPerms, perms) {
		boom(t, "for x/y/z got %v, don't want %v", perms, rootPerms)
	}
}

func TestLimits(t *testing.T) {
	ctx, _, _, shutdown := initTest()
	defer shutdown()

	stop, estr, bt, _ := newMT(t, "", ctx, &internal.Config{MaxNodesPerUser: 10, MaxServersPerUser: 15})
	defer stop()

	fakeServer := naming.JoinAddressName(estr, "quux")

	checkCounters := func(n, s int) {
		counters, err := bt.Counters(ctx)
		if err != nil {
			t.Fatalf("bt.Counters failed: %v", err)
		}
		numNodes, numServers := counters["num-nodes-per-user:root"], counters["num-servers-per-user:root"]
		if numNodes != int64(n) {
			t.Errorf("Unexpected number of nodes. Got %d, expected %d", numNodes, n)
		}
		if numServers != int64(s) {
			t.Errorf("Unexpected number of servers. Got %d, expected %d", numServers, s)
		}
	}

	// User is allowed to created 10 nodes.
	for i := 1; i <= 10; i++ {
		doMount(t, ctx, estr, fmt.Sprintf("%d", i), fakeServer, true)
		checkCounters(i, i)
	}
	// But not 11.
	doMount(t, ctx, estr, "foo", fakeServer, false)
	checkCounters(10, 10)

	// User is allowed to mount 15 servers.
	for i := 1; i <= 5; i++ {
		doMount(t, ctx, estr, fmt.Sprintf("%d", i), naming.Join(fakeServer, "foo"), true)
		checkCounters(10, 10+i)
	}
	// But not 16.
	doMount(t, ctx, estr, "6", naming.Join(fakeServer, "foo"), false)
	checkCounters(10, 15)

	// Freeing one node allows to create one.
	doUnmount(t, ctx, estr, "10", "", true)
	checkCounters(9, 14)
	doMount(t, ctx, estr, "10", fakeServer, true)
	checkCounters(10, 15)

	// Unmount everything.
	for i := 1; i <= 10; i++ {
		doUnmount(t, ctx, estr, fmt.Sprintf("%d", i), "", true)
		if i <= 5 {
			checkCounters(10-i, 15-2*i)
		} else {
			checkCounters(10-i, 10-i)
		}
	}
}

func initTest() (rootCtx *context.T, aliceCtx *context.T, bobCtx *context.T, shutdown v23.Shutdown) {
	ctx, shutdown := test.V23InitWithMounttable()
	var err error
	if rootCtx, err = v23.WithPrincipal(ctx, testutil.NewPrincipal("root")); err != nil {
		panic("failed to set root principal")
	}
	if aliceCtx, err = v23.WithPrincipal(ctx, testutil.NewPrincipal("alice")); err != nil {
		panic("failed to set alice principal")
	}
	if bobCtx, err = v23.WithPrincipal(ctx, testutil.NewPrincipal("bob")); err != nil {
		panic("failed to set bob principal")
	}
	for _, r := range []*context.T{rootCtx, aliceCtx, bobCtx} {
		// A hack to set the namespace roots to a value that won't work.
		v23.GetNamespace(r).SetRoots()
		// And have all principals recognize each others blessings.
		p1 := v23.GetPrincipal(r)
		for _, other := range []*context.T{rootCtx, aliceCtx, bobCtx} {
			// testutil.NewPrincipal has already setup each
			// principal to use the same blessing for both server
			// and client activities.
			bother, _ := v23.GetPrincipal(other).BlessingStore().Default()
			if err := security.AddToRoots(p1, bother); err != nil {
				panic(err)
			}
		}
	}
	return rootCtx, aliceCtx, bobCtx, shutdown
}
