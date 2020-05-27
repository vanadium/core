// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package namespace_test

import (
	"fmt"
	"reflect"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func initTest() (rootCtx *context.T, aliceCtx *context.T, bobCtx *context.T, shutdown v23.Shutdown) {
	ctx, shutdown := test.V23Init()
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
		if err := v23.GetNamespace(r).SetRoots(); err != nil {
			panic(err)
		}
		// And have all principals recognize each others blessings.
		p1 := v23.GetPrincipal(r)
		for _, other := range []*context.T{rootCtx, aliceCtx, bobCtx} {
			// testutil.NewPrincipal has already setup each
			// principal to use the same blessing for both server
			// and client activities.
			b, _ := v23.GetPrincipal(other).BlessingStore().Default()
			if err := security.AddToRoots(p1, b); err != nil {
				panic(err)
			}
		}
	}
	return rootCtx, aliceCtx, bobCtx, shutdown
}

// Create a new mounttable service.
func newMT(t *testing.T, ctx *context.T) (func(), string) {
	estr, stopFunc, err := mounttablelib.StartServers(ctx, v23.GetListenSpec(ctx), "", "", "", "", "mounttable")
	if err != nil {
		t.Fatalf("r.NewServer: %s", err)
	}
	return stopFunc, estr
}

type nopServer struct{ x int }

func (s *nopServer) NOP(*context.T, rpc.ServerCall) error {
	return nil
}

var nobody = []security.BlessingPattern{""}
var everybody = []security.BlessingPattern{"..."}
var closedPerms = access.Permissions{
	"Resolve": access.AccessList{
		In: nobody,
	},
	"Read": access.AccessList{
		In: nobody,
	},
	"Admin": access.AccessList{
		In: nobody,
	},
	"Create": access.AccessList{
		In: nobody,
	},
	"Mount": access.AccessList{
		In: nobody,
	},
}
var closedPermsWithOwnerAdded = access.Permissions{
	"Resolve": access.AccessList{
		In: nobody,
	},
	"Read": access.AccessList{
		In: nobody,
	},
	"Admin": access.AccessList{
		In: []security.BlessingPattern{"", "root"},
	},
	"Create": access.AccessList{
		In: nobody,
	},
	"Mount": access.AccessList{
		In: nobody,
	},
}
var openPerms = access.Permissions{
	"Resolve": access.AccessList{
		In: everybody,
	},
	"Read": access.AccessList{
		In: everybody,
	},
	"Admin": access.AccessList{
		In: everybody,
	},
	"Create": access.AccessList{
		In: everybody,
	},
	"Mount": access.AccessList{
		In: everybody,
	},
}

func TestPermissions(t *testing.T) { //nolint:gocyclo
	// Create three different personalities.
	// TODO(p): Use the multiple personalities to test Permissions functionality.
	rootCtx, aliceCtx, _, shutdown := initTest()
	defer shutdown()

	// Create root mounttable.
	stop, rmtAddr := newMT(t, rootCtx)
	fmt.Printf("rmt at %s\n", rmtAddr)
	defer stop()
	ns := v23.GetNamespace(rootCtx)
	if err := ns.SetRoots("/" + rmtAddr); err != nil {
		t.Fatal(err)
	}

	// Create lower mount table.
	stop1, mt1Addr := newMT(t, rootCtx)
	fmt.Printf("mt1 at %s\n", mt1Addr)
	defer stop1()

	// Mount them into the root.
	if err := ns.Mount(rootCtx, "a/b/c", mt1Addr, 0, naming.ServesMountTable(true)); err != nil {
		t.Fatalf("Failed to Mount %s onto a/b/c: %s", "/"+mt1Addr, err)
	}

	// Set/Get the mount point's Permissions.
	_, version, err := ns.GetPermissions(rootCtx, "a/b/c")
	if err != nil {
		t.Fatalf("GetPermissions a/b/c: %s", err)
	}
	if err := ns.SetPermissions(rootCtx, "a/b/c", openPerms, version); err != nil {
		t.Fatalf("SetPermissions a/b/c: %s", err)
	}
	nacl, _, err := ns.GetPermissions(rootCtx, "a/b/c")
	if err != nil {
		t.Fatalf("GetPermissions a/b/c: %s", err)
	}
	if !reflect.DeepEqual(openPerms, nacl) {
		t.Fatalf("want %v, got %v", openPerms, nacl)
	}

	// Now Set/Get the in lower mount table.
	name := "a/b/c/d/e"
	version = "" // Parallel setperms with any other value is dangerous
	if err := ns.SetPermissions(rootCtx, name, openPerms, version); err != nil {
		t.Fatalf("SetPermissions %s: %s", name, err)
	}
	nacl, _, err = ns.GetPermissions(rootCtx, name)
	if err != nil {
		t.Fatalf("GetPermissions %s: %s", name, err)
	}
	if !reflect.DeepEqual(openPerms, nacl) {
		t.Fatalf("want %v, got %v", openPerms, nacl)
	}

	// Create mount points accessible only by root's key and owner.
	name = "a/b/c/d/f"
	deadbody := "/the:8888/rain"
	if err := ns.SetPermissions(rootCtx, name, closedPerms, version); err != nil {
		t.Fatalf("SetPermissions %s: %s", name, err)
	}
	nacl, _, err = ns.GetPermissions(rootCtx, name)
	if err != nil {
		t.Fatalf("GetPermissions %s: %s", name, err)
	}
	if !reflect.DeepEqual(closedPermsWithOwnerAdded, nacl) {
		t.Fatalf("want %v, got %v", closedPermsWithOwnerAdded, nacl)
	}
	if err := ns.Mount(rootCtx, name, deadbody, 10000); err != nil {
		t.Fatalf("Mount %s: %s", name, err)
	}

	// Alice shouldn't be able to resolve it.
	_, err = v23.GetNamespace(aliceCtx).Resolve(aliceCtx, name)
	if err == nil {
		t.Fatalf("as alice we shouldn't be able to Resolve %s", name)
	}

	// Root should be able to resolve it.
	_, err = ns.Resolve(rootCtx, name)
	if err != nil {
		t.Fatalf("as root Resolve %s: %s", name, err)
	}

	// Create a mount point via Serve accessible only by root's key and owner.
	name = "a/b/c/d/g"
	if err := ns.SetPermissions(rootCtx, name, closedPerms, version); err != nil {
		t.Fatalf("SetPermissions %s: %s", name, err)
	}
	if rootCtx, _, err = v23.WithNewServer(rootCtx, name, &nopServer{1}, nil); err != nil {
		t.Fatalf("v23.WithNewServer failed: %v", err)
	}
	// Alice shouldn't be able to resolve it.
	_, err = v23.GetNamespace(aliceCtx).Resolve(aliceCtx, name)
	if err == nil {
		t.Fatalf("as alice we shouldn't be able to Resolve %s", name)
	}

	// Root should be able to resolve it.
	resolveWithRetry(rootCtx, name)
}
