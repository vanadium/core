// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"reflect"
	"sort"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/namespace"
	"v.io/v23/rpc"

	"v.io/x/ref/lib/publisher"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

func resolveWithRetry(t *testing.T, ns namespace.T, ctx *context.T, name string, expected int) []string {
	deadline := time.Now().Add(10 * time.Second)
	for {
		me, err := ns.Resolve(ctx, name)
		if err == nil && len(me.Names()) == expected {
			return me.Names()
		}
		if time.Now().After(deadline) {
			t.Fatalf("failed to resolve %q", name)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func verifyMissing(t *testing.T, ns namespace.T, ctx *context.T, name string) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := ns.Resolve(ctx, "foo"); err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%q is still mounted", name)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestAddAndRemove(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	ns := v23.GetNamespace(ctx)
	pubctx, cancel := context.WithCancel(ctx)
	pub := publisher.New(pubctx, ns, time.Second)
	pub.AddName("foo", false, false)
	pub.AddServer("foo:8000")
	if got, want := resolveWithRetry(t, ns, ctx, "foo", 1), []string{"/foo:8000"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	pub.AddServer("bar:8000")
	got, want := resolveWithRetry(t, ns, ctx, "foo", 2), []string{"/bar:8000", "/foo:8000"}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	pub.AddName("baz", false, false)
	got = resolveWithRetry(t, ns, ctx, "baz", 2)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	pub.RemoveName("foo")
	verifyMissing(t, ns, ctx, "foo")

	cancel()
	<-pub.Closed()
}

func TestStatus(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	ns := v23.GetNamespace(ctx)
	pubctx, cancel := context.WithCancel(ctx)
	pub := publisher.New(pubctx, ns, time.Second)
	pub.AddName("foo", false, false)
	status, _ := pub.Status()
	if got, want := len(status), 0; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	pub.AddServer("foo:8000")

	// Wait for the publisher to asynchronously publish the
	// requisite number of servers.
	waitFor := func(n int) {
		for {
			status, dirty := pub.Status()
			if got, want := len(status), n; got != want {
				<-dirty
			} else {
				return
			}
		}
	}

	waitFor(1)

	pub.AddServer("bar:8000")
	pub.AddName("baz", false, false)

	waitFor(4)

	status, _ = pub.Status()
	names, servers := publisherNamesAndServers(status)
	// There will be two of each name and two of each server in the mount entries.
	if got, want := names, []string{"baz", "baz", "foo", "foo"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := servers, []string{"bar:8000", "bar:8000", "foo:8000", "foo:8000"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	pub.RemoveName("foo")
	waitFor(2)
	verifyMissing(t, ns, ctx, "foo")

	status, _ = pub.Status()
	names, servers = publisherNamesAndServers(status)
	if got, want := names, []string{"baz", "baz"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := servers, []string{"bar:8000", "foo:8000"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	cancel()
	<-pub.Closed()
}

func TestRemoveFailedAdd(t *testing.T) {
	// Test that removing an already unmounted name (due to an error while mounting),
	// results in a Status that has no entries.
	// We use test.V23Init instead of test.V23InitWithMounTable to make all
	// calls to the mounttable fail: the context returned by test.V23Init
	// has no namespace roots.
	ctx, shutdown := test.V23Init()
	defer shutdown()
	ns := v23.GetNamespace(ctx)
	pubctx, cancel := context.WithCancel(ctx)
	pub := publisher.New(pubctx, ns, time.Second)
	pub.AddServer("foo:8000")
	// Adding a name should result in one entry in the publisher with state PublisherMounting, since
	// it can never successfully mount.
	pub.AddName("foo", false, false)
	status, _ := pub.Status()
	if got, want := len(status), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	// We want to ensure that the LastState is either:
	// PublisherUnmounted if the publisher hasn't tried to mount yet, or
	// PublisherMounting if the publisher tried to mount but failed.
	if got, want := status[0].LastState, rpc.PublisherMounting; got > want {
		t.Fatalf("got %s, want %s", got, want)
	}
	// Removing "foo" should result in an empty Status.
	pub.RemoveName("foo")
	for {
		status, dirty := pub.Status()
		if got, want := len(status), 0; got != want {
			<-dirty
		} else {
			break
		}
	}

	cancel()
	<-pub.Closed()
}

func publisherNamesAndServers(entries []rpc.PublisherEntry) (names []string, servers []string) {
	for _, e := range entries {
		names = append(names, e.Name)
		servers = append(servers, e.Server)
	}
	sort.Strings(names)
	sort.Strings(servers)
	return names, servers
}
