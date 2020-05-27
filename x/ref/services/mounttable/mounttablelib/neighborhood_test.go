// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mounttablelib_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/naming"
	"v.io/v23/options"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/test"
)

//nolint:deadcode,unused
type stopper interface {
	Stop()
}

func TestNeighborhood(t *testing.T) { //nolint:gocyclo
	t.Skip("This test is timeing out due the mdns code hanging whilst closing a UDP connection - see the defer nhd.(stopper).Stop() line below")
	rootCtx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	rootCtx.Infof("TestNeighborhood")

	mstr := v23.GetNamespace(rootCtx).Roots()[0]
	addresses := []string{
		naming.JoinAddressName(mstr, ""),
		naming.JoinAddressName(mstr, "suffix1"),
		naming.JoinAddressName(mstr, "suffix2"),
	}

	// Create a name for the server.
	serverName := fmt.Sprintf("nhtest%d", os.Getpid())
	// Add neighborhood server.
	nhd, err := mounttablelib.NewLoopbackNeighborhoodDispatcher(serverName, addresses...)
	if err != nil {
		boom(t, "Failed to create neighborhood server: %s\n", err)
	}
	// This hangs...
	defer nhd.(stopper).Stop()

	// Start serving on a loopback address.
	rootCtx, server, err := v23.WithNewDispatchingServer(rootCtx, "", nhd)
	if err != nil {
		boom(t, "Failed to create neighborhood: %s", err)
	}
	estr := server.Status().Endpoints[0].Name()

	// Wait for the mounttable to appear in mdns
L:
	for tries := 1; tries < 2; tries++ {
		names := doGlob(t, rootCtx, estr, "", "*")
		t.Logf("names %v", names)
		for _, n := range names {
			if n == serverName {
				break L
			}
		}
		time.Sleep(1 * time.Second)
	}

	// Make sure we get back a root for the server.
	want, got := []string{""}, doGlob(t, rootCtx, estr, serverName, "")
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Unexpected Glob result want: %q, got: %q", want, got)
	}

	// Make sure we can resolve through the neighborhood.
	expectedSuffix := "a/b"

	client := v23.GetClient(rootCtx)
	name := naming.JoinAddressName(estr, serverName+"/"+expectedSuffix)
	call, cerr := client.StartCall(rootCtx, name, "ResolveStep", nil, options.Preresolved{})
	if cerr != nil {
		boom(t, "ResolveStep.StartCall: %s", cerr)
	}
	var entry naming.MountEntry
	if err := call.Finish(&entry); err != nil {
		boom(t, "ResolveStep: %s", err)
	}

	// Resolution returned something.  Make sure its correct.
	if entry.Name != expectedSuffix {
		boom(t, "resolveStep suffix: expected %s, got %s", expectedSuffix, entry.Name)
	}
	if len(entry.Servers) == 0 {
		boom(t, "resolveStep returns no severs")
	}
L2:
	for _, s := range entry.Servers {
		for _, a := range addresses {
			if a == s.Server {
				continue L2
			}
		}
		boom(t, "Unexpected address from resolveStep result: %v", s.Server)
	}
L3:
	for _, a := range addresses {
		for _, s := range entry.Servers {
			if a == s.Server {
				continue L3
			}
		}
		boom(t, "Missing address from resolveStep result: %v", a)
	}
}
