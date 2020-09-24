// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	vsecurity "v.io/x/ref/lib/security"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/role"
	irole "v.io/x/ref/services/role/roled/internal"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func TestSeekBlessings(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	workdir, err := ioutil.TempDir("", "test-role-server-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)

	// Role A is a restricted role, i.e. it can be used in sensitive Permissions.
	roleAConf := irole.Config{
		Members: []security.BlessingPattern{
			"test-blessing:users:user1:_role",
			"test-blessing:users:user2:_role",
			"test-blessing:users:user3", // _role implied
		},
		Extend: true,
	}
	irole.WriteConfig(t, roleAConf, filepath.Join(workdir, "A.conf"))

	// Role B is an unrestricted role.
	roleBConf := irole.Config{
		Members: []security.BlessingPattern{
			"test-blessing:users:user1:_role",
			"test-blessing:users:user3:_role",
		},
		Audit:  true,
		Extend: false,
	}
	irole.WriteConfig(t, roleBConf, filepath.Join(workdir, "B.conf"))

	// Role C/D for testing out nesting.
	roleCConf := irole.Config{
		Members: []security.BlessingPattern{"test-blessing:users:user1:_role"},
		Extend:  true,
	}
	if err := os.Mkdir(filepath.Join(workdir, "C"), 0755); err != nil {
		t.Fatal(err)
	}
	irole.WriteConfig(t, roleCConf, filepath.Join(workdir, "C", "D.conf"))

	root := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))

	var (
		user1  = newPrincipalContext(t, ctx, root, "users:user1")
		user1R = newPrincipalContext(t, ctx, root, "users:user1:_role")
		user2  = newPrincipalContext(t, ctx, root, "users:user2")
		user2R = newPrincipalContext(t, ctx, root, "users:user2:_role")
		user3  = newPrincipalContext(t, ctx, root, "users:user3")
		user3R = newPrincipalContext(t, ctx, root, "users:user3", "users:user3:_role:foo", "users:user3:_role:bar")
	)

	testServerCtx := newPrincipalContext(t, ctx, root, "testserver")
	tDisp := &testDispatcher{}
	_, _, err = v23.WithNewDispatchingServer(testServerCtx, "test", tDisp)
	if err != nil {
		t.Fatalf("NewDispatchingServer failed: %v", err)
	}

	var noErr error
	testcases := []struct {
		ctx       *context.T
		role      string
		errID     error
		blessings []string
	}{
		{user1, "", verror.ErrUnknownMethod, nil},
		{user1, "unknown", verror.ErrNoAccess, nil},
		{user2, "unknown", verror.ErrNoAccess, nil},
		{user3, "unknown", verror.ErrNoAccess, nil},

		{user1, "A", verror.ErrNoAccess, nil},
		{user1R, "A", noErr, []string{"test-blessing:roles:A:test-blessing:users:user1"}},
		{user2, "A", verror.ErrNoAccess, nil},
		{user2R, "A", noErr, []string{"test-blessing:roles:A:test-blessing:users:user2"}},
		{user3, "A", verror.ErrNoAccess, nil},
		{user3R, "A", noErr, []string{"test-blessing:roles:A:test-blessing:users:user3:_role:bar", "test-blessing:roles:A:test-blessing:users:user3:_role:foo"}},

		{user1, "B", verror.ErrNoAccess, nil},
		{user1R, "B", noErr, []string{"test-blessing:roles:B"}},
		{user2, "B", verror.ErrNoAccess, nil},
		{user2R, "B", verror.ErrNoAccess, nil},
		{user3, "B", verror.ErrNoAccess, nil},
		{user3R, "B", noErr, []string{"test-blessing:roles:B"}},

		{user1R, "C/D", noErr, []string{"test-blessing:roles:C:D:test-blessing:users:user1"}},
	}
	addr := newRoleServer(t, newPrincipalContext(t, ctx, root, "roles"), workdir)
	for _, tc := range testcases {
		user, _ := v23.GetPrincipal(tc.ctx).BlessingStore().Default()
		c := role.RoleClient(naming.Join(addr, tc.role))
		blessings, err := c.SeekBlessings(tc.ctx)
		if !errors.Is(err, tc.errID) {
			t.Errorf("unexpected error ID for (%q, %q). Got %#v, expected %#v", user, tc.role, verror.ErrorID(err), tc.errID)
		}
		if err != nil {
			continue
		}
		previousBlessings, _ := v23.GetPrincipal(tc.ctx).BlessingStore().Set(blessings, security.AllPrincipals)
		blessingNames, rejected := callTest(t, tc.ctx, "test")
		if !reflect.DeepEqual(blessingNames, tc.blessings) {
			t.Errorf("unexpected blessings for (%q, %q). Got %q, expected %q", user, tc.role, blessingNames, tc.blessings)
		}
		if len(rejected) != 0 {
			t.Errorf("unexpected rejected blessings for (%q, %q): %q", user, tc.role, rejected)
		}
		if _, err := v23.GetPrincipal(tc.ctx).BlessingStore().Set(previousBlessings, security.AllPrincipals); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPeerBlessingCaveats(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	workdir, err := ioutil.TempDir("", "test-role-server-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)

	roleConf := irole.Config{
		Members: []security.BlessingPattern{"test-blessing:users:user:_role"},
		Peers: []security.BlessingPattern{
			security.BlessingPattern("test-blessing:peer1"),
			security.BlessingPattern("test-blessing:peer3"),
		},
	}
	irole.WriteConfig(t, roleConf, filepath.Join(workdir, "role.conf"))

	var (
		root  = testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
		user  = newPrincipalContext(t, ctx, root, "users:user:_role")
		peer1 = newPrincipalContext(t, ctx, root, "peer1")
		peer2 = newPrincipalContext(t, ctx, root, "peer2")
		peer3 = newPrincipalContext(t, ctx, root, "peer3")
	)

	roleAddr := newRoleServer(t, newPrincipalContext(t, ctx, root, "roles"), workdir)

	tDisp := &testDispatcher{}
	_, _, err = v23.WithNewDispatchingServer(peer1, "peer1", tDisp)
	if err != nil {
		t.Fatalf("NewDispatchingServer failed: %v", err)
	}
	_, _, err = v23.WithNewDispatchingServer(peer2, "peer2", tDisp)
	if err != nil {
		t.Fatalf("NewDispatchingServer failed: %v", err)
	}
	_, _, err = v23.WithNewDispatchingServer(peer3, "peer3", tDisp)
	if err != nil {
		t.Fatalf("NewDispatchingServer failed: %v", err)
	}

	c := role.RoleClient(naming.Join(roleAddr, "role"))
	blessings, err := c.SeekBlessings(user)
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if _, err := v23.GetPrincipal(user).BlessingStore().Set(blessings, security.AllPrincipals); err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		peer          string
		blessingNames []string
		rejectedNames []string
	}{
		{"peer1", []string{"test-blessing:roles:role"}, nil},
		{"peer2", nil, []string{"test-blessing:roles:role"}},
		{"peer3", []string{"test-blessing:roles:role"}, nil},
	}
	for i, tc := range testcases {
		blessingNames, rejected := callTest(t, user, tc.peer)
		var rejectedNames []string
		for _, r := range rejected {
			rejectedNames = append(rejectedNames, r.Blessing)
		}
		if !reflect.DeepEqual(blessingNames, tc.blessingNames) {
			t.Errorf("Unexpected blessing names for #%d. Got %q, expected %q", i, blessingNames, tc.blessingNames)
		}
		if !reflect.DeepEqual(rejectedNames, tc.rejectedNames) {
			t.Errorf("Unexpected rejected names for #%d. Got %q, expected %q", i, rejectedNames, tc.rejectedNames)
		}
	}
}

func TestGlob(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	workdir, err := ioutil.TempDir("", "test-role-server-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)
	os.Mkdir(filepath.Join(workdir, "sub1"), 0700)         //nolint:errcheck
	os.Mkdir(filepath.Join(workdir, "sub1", "sub2"), 0700) //nolint:errcheck
	os.Mkdir(filepath.Join(workdir, "sub3"), 0700)         //nolint:errcheck

	// Role that user1 has access to.
	roleAConf := irole.Config{Members: []security.BlessingPattern{"test-blessing:user1"}}
	irole.WriteConfig(t, roleAConf, filepath.Join(workdir, "A.conf"))
	irole.WriteConfig(t, roleAConf, filepath.Join(workdir, "sub1/B.conf"))
	irole.WriteConfig(t, roleAConf, filepath.Join(workdir, "sub1/C.conf"))
	irole.WriteConfig(t, roleAConf, filepath.Join(workdir, "sub1/sub2/D.conf"))

	// Role that user2 has access to.
	roleBConf := irole.Config{Members: []security.BlessingPattern{"test-blessing:user2"}}
	irole.WriteConfig(t, roleBConf, filepath.Join(workdir, "sub1/sub2/X.conf"))

	root := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
	user1 := newPrincipalContext(t, ctx, root, "user1:_role")
	user2 := newPrincipalContext(t, ctx, root, "user2:_role")
	user3 := newPrincipalContext(t, ctx, root, "user3:_role")
	addr := newRoleServer(t, newPrincipalContext(t, ctx, root, "roles"), workdir)

	testcases := []struct {
		user    *context.T
		name    string
		pattern string
		results []string
	}{
		{user1, "", "*", []string{"A", "sub1"}},
		{user1, "sub1", "*", []string{"B", "C", "sub2"}},
		{user1, "sub1/sub2", "*", []string{"D"}},
		{user1, "", "...", []string{"", "A", "sub1", "sub1/B", "sub1/C", "sub1/sub2", "sub1/sub2/D"}},
		{user2, "", "*", []string{"sub1"}},
		{user2, "", "...", []string{"", "sub1", "sub1/sub2", "sub1/sub2/X"}},
		{user3, "", "*", []string{}},
		{user3, "", "...", []string{""}},
	}
	for i, tc := range testcases {
		matches, _, _ := testutil.GlobName(tc.user, naming.Join(addr, tc.name), tc.pattern)
		if !reflect.DeepEqual(matches, tc.results) {
			t.Errorf("unexpected results for tc #%d. Got %q, expected %q", i, matches, tc.results)
		}
	}
}

func newPrincipalContext(t *testing.T, ctx *context.T, root *testutil.IDProvider, names ...string) *context.T {
	principal := testutil.NewPrincipal()
	var blessings []security.Blessings
	for _, n := range names {
		blessing, err := root.NewBlessings(principal, n)
		if err != nil {
			t.Fatalf("root.Bless failed for %q: %v", n, err)
		}
		blessings = append(blessings, blessing)
	}
	bUnion, err := security.UnionOfBlessings(blessings...)
	if err != nil {
		t.Fatalf("security.UnionOfBlessings failed: %v", err)
	}
	if err := vsecurity.SetDefaultBlessings(principal, bUnion); err != nil {
		t.Fatal(err)
	}
	ctx, err = v23.WithPrincipal(ctx, principal)
	if err != nil {
		t.Fatalf("v23.WithPrincipal failed: %v", err)
	}
	return ctx
}

func newRoleServer(t *testing.T, ctx *context.T, dir string) string {
	_, _, err := v23.WithNewDispatchingServer(ctx, "role", irole.NewDispatcher(dir, "role"))
	if err != nil {
		t.Fatalf("ServeDispatcher failed: %v", err)
	}
	return "role"
}

func callTest(t *testing.T, ctx *context.T, addr string) (blessingNames []string, rejected []security.RejectedBlessing) {
	call, err := v23.GetClient(ctx).StartCall(ctx, addr, "Test", nil)
	if err != nil {
		t.Fatalf("StartCall failed: %v", err)
	}
	if err := call.Finish(&blessingNames, &rejected); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	return
}

type testDispatcher struct {
}

func (d *testDispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return d, d, nil
}

func (d *testDispatcher) Authorize(*context.T, security.Call) error {
	return nil
}

func (d *testDispatcher) Test(ctx *context.T, call rpc.ServerCall) ([]string, []security.RejectedBlessing, error) {
	blessings, rejected := security.RemoteBlessingNames(ctx, call.Security())
	return blessings, rejected, nil
}
