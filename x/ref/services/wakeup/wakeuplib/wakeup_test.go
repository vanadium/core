// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wakeuplib

import (
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/wakeup"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func newWakeup(t *testing.T, ctx *context.T, wakeup func(*context.T, string, string) error) func() {
	// Mint new shared key.
	var sharedKey [32]byte
	if _, err := crand.Read(sharedKey[:]); err != nil {
		t.Fatalf("Failed to create wakeup server: %v", err)
	}
	stop, err := StartServers(ctx, "wakeup", "", sharedKey, wakeup)
	if err != nil {
		t.Fatalf("Failed to start wakeup server: %v", err)
	}
	return stop
}

func newEndpoint(t *testing.T, addr string) string {
	ep := naming.Endpoint{Address: addr}
	return ep.String()
}

func register(t *testing.T, ctx *context.T, token string) string {
	name, err := wakeup.WakeUpClient("wakeup/server").Register(ctx, token)
	if err != nil {
		t.Fatalf("Register(%s) failed: %v", token, err)
	}
	return name
}

func mount(t *testing.T, ctx *context.T, name, ep string, shouldSucceed bool) {
	if err := v23.GetNamespace(ctx).Mount(ctx, name, ep, 1*time.Minute); err != nil {
		if !shouldSucceed {
			return
		}
		t.Fatalf("Mount(%s) failed: %v", name, err)
	}
}

func resolve(t *testing.T, ctx *context.T, name string, shouldSucceed bool) string {
	entry, err := v23.GetNamespace(ctx).Resolve(ctx, name)
	if err != nil {
		if !shouldSucceed {
			return ""
		}
		t.Fatalf("Resolve(%s) failed: %v", name, err)
	}
	if len(entry.Servers) != 1 {
		t.Fatalf("Expected a single mounted server, got: %v", entry.Servers)
	}
	return entry.Servers[0].Server
}

func setPermissions(t *testing.T, ctx *context.T, name string, perms access.Permissions, shouldSucceed bool) {
	if err := v23.GetNamespace(ctx).SetPermissions(ctx, name, perms, ""); err != nil {
		if shouldSucceed {
			t.Fatalf("SetPermissions(%s) failed: %v", name, err)
		}
	}
	if !shouldSucceed {
		t.Fatalf("SetPermissions(%s) should have failed", name)
	}
}

func getPermissions(t *testing.T, ctx *context.T, name string, want access.Permissions) {
	got, _, err := v23.GetNamespace(ctx).GetPermissions(ctx, name)
	if err != nil {
		t.Fatalf("GetPermissions(%s) failed: %v", name, err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("Want permissions %v, got %v", want, got)
	}
}

func checkWokenUp(t *testing.T, want string, wakeupCh <-chan string, shouldSucceed bool) {
	select {
	case got := <-wakeupCh:
		if !shouldSucceed {
			t.Fatalf("Server wrongfully woken up")
		} else if got != want {
			t.Fatalf("woken up with invalid token, got %v want %v", got, want)
		} else {
			select {
			case val := <-wakeupCh:
				t.Fatalf("Wakeup channel should be empty, found value %v", val)
			default:
				// OK
			}
		}
	default:
		if shouldSucceed {
			t.Fatalf("never woken up")
		}
	}
}

func withNewPrincipal(t *testing.T, ctx *context.T) *context.T {
	p := testutil.NewPrincipal()
	d, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	b, err := v23.GetPrincipal(ctx).Bless(p.PublicKey(), d, fmt.Sprintf("derived%d", rand.Int()), security.UnconstrainedUse())
	if err != nil {
		t.Fatalf("Error creating principal: %v", err)
	}
	if err := security.AddToRoots(p, b); err != nil {
		t.Fatalf("Couldn't add %v to principal %v's roots: %v", b, p, err)
	}
	if err := p.BlessingStore().SetDefault(b); err != nil {
		t.Fatalf("Couldn't set default blessing %v for principal %v: %v", b, p, err)
	}
	if _, err := p.BlessingStore().Set(b, security.AllPrincipals); err != nil {
		t.Fatalf("Couldn't set blessing %v for all peers of principal %v: %v", b, p, err)
	}
	ret, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		t.Fatalf("Couldn't create new context for principal %v: %v", p, err)
	}
	return ret
}

func TestWakeup(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	wakeupCh := make(chan string, 100)
	stop := newWakeup(t, ctx, func(ctx *context.T, token, _ string) error {
		wakeupCh <- token
		return nil
	})
	defer stop()
	// One.
	aRoot := register(t, ctx, "aToken")
	aMountName := naming.Join(aRoot, "a/server/aa/aaa")
	aWakeupName := naming.Join(aRoot, "a/client/aa/aaa")
	aEndpoint := newEndpoint(t, "aAddr:1")
	mount(t, ctx, aMountName, aEndpoint, true)
	checkWokenUp(t, "aToken", wakeupCh, false)
	if want, got := aEndpoint, resolve(t, ctx, aWakeupName, true); want != got {
		t.Errorf("Resolve: got %v want %v", got, want)
	}
	checkWokenUp(t, "aToken", wakeupCh, true)
	perms := mtPerms("...")
	setPermissions(t, ctx, naming.Join(aRoot, "a/server/aa"), perms, true)
	getPermissions(t, ctx, naming.Join(aRoot, "a/client/aa"), perms)
	checkWokenUp(t, "aToken", wakeupCh, true)
	// Two.
	bRoot := register(t, ctx, "bToken")
	bMountName := naming.Join(bRoot, "b/server")
	bWakeupName := naming.Join(bRoot, "b/client")
	bEndpoint := newEndpoint(t, "bAddr:2")
	mount(t, ctx, bMountName, bEndpoint, true)
	checkWokenUp(t, "bToken", wakeupCh, false)
	if want, got := bEndpoint, resolve(t, ctx, bWakeupName, true); want != got {
		t.Errorf("Resolve: got %v want %v", got, want)
	}
	checkWokenUp(t, "bToken", wakeupCh, true)
	// Fail.
	cRoot := register(t, ctx, "cToken")
	cMountName := naming.Join(cRoot, "c/server")
	resolve(t, ctx, cMountName, false) // not mounted
	checkWokenUp(t, "cToken", wakeupCh, false)
	// Restricted set of resolvable names.
	resolve(t, ctx, "wakeup/server/foo", false)
	resolve(t, ctx, "wakeup/mounts", false)
	resolve(t, ctx, "wakeup/mounts/foo/bar", false)
	resolve(t, ctx, "wakeup/foo", false)
}

func TestWakeupPermissions(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	ctxOne := withNewPrincipal(t, ctx)
	ctxTwo := withNewPrincipal(t, ctx)

	wakeupCh := make(chan string, 100)
	stop := newWakeup(t, ctxOne, func(ctx *context.T, token, _ string) error {
		wakeupCh <- token
		return nil
	})
	defer stop()

	// Success.
	aRoot := register(t, ctxTwo, "aToken")
	aMountName := naming.Join(aRoot, "a/server/aa/aaa")
	aWakeupName := naming.Join(aRoot, "a/client/aa/aaa")
	aEndpoint := newEndpoint(t, "aAddr:1")
	mount(t, ctxTwo, aMountName, aEndpoint, true)
	checkWokenUp(t, "aToken", wakeupCh, false)
	if want, got := aEndpoint, resolve(t, ctxTwo, aWakeupName, true); want != got {
		t.Errorf("Resolve: got %v want %v", got, want)
	}
	checkWokenUp(t, "aToken", wakeupCh, true)
	// Failure.
	bRoot := register(t, ctxOne, "bToken")
	bMountName := naming.Join(bRoot, "b/server")
	bWakeupName := naming.Join(bRoot, "b/client")
	bEndpoint := newEndpoint(t, "bAddr:2")
	mount(t, ctxTwo, bMountName, bEndpoint, false)
	mount(t, ctxOne, bMountName, bEndpoint, true)
	checkWokenUp(t, "bToken", wakeupCh, false)
	if want, got := bEndpoint, resolve(t, ctxTwo, bWakeupName, true); want != got {
		t.Errorf("Resolve: got %v want %v", got, want)
	}
	checkWokenUp(t, "bToken", wakeupCh, true)
	cRoot := register(t, ctxOne, "cToken")
	cMountName := naming.Join(cRoot, "c/server/cc/ccc")
	cWakeupName := naming.Join(cRoot, "c/client/cc/ccc")
	cEndpoint := newEndpoint(t, "cAddr:3")
	mount(t, ctxTwo, cMountName, cEndpoint, false)
	mount(t, ctxOne, cMountName, cEndpoint, true)
	if want, got := cEndpoint, resolve(t, ctxTwo, cWakeupName, true); want != got {
		t.Errorf("Resolve: got %v want %v", got, want)
	}
}
