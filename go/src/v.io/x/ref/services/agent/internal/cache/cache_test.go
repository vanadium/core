// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/test/testutil"
)

func createRoots() ([]byte, security.BlessingRoots, *cachedRoots) {
	var mu sync.RWMutex
	ctx, _ := context.RootContext()
	ctx = context.WithLogger(ctx, logger.Global())
	p := testutil.NewPrincipal()
	impl := p.Roots()
	roots, err := newCachedRoots(impl, &mu)
	if err != nil {
		panic(err)
	}
	keybytes, err := p.PublicKey().MarshalBinary()
	if err != nil {
		panic(err)
	}
	return keybytes, impl, roots
}

func TestCreateRoots(t *testing.T) {
	_, impl, cache := createRoots()
	if impl == security.BlessingRoots(cache) {
		t.Fatalf("Same roots")
	}
	if impl == nil || cache == nil {
		t.Fatalf("No roots %v %v", impl, cache)
	}
}

func expectRecognized(roots security.BlessingRoots, key []byte, blessing string) string {
	if err := roots.Recognized(key, blessing); err != nil {
		return fmt.Sprintf("Key (%s, %v) not matched by roots:\n%s, Recognized returns error: %v", key, blessing, roots.DebugString(), err)
	}
	return ""
}

func expectNotRecognized(roots security.BlessingRoots, key []byte, blessing string) string {
	if err := roots.Recognized(key, blessing); err == nil {
		return fmt.Sprintf("Key (%s, %s) should not match roots:\n%s", key, blessing, roots.DebugString())
	}
	return ""
}

func TestAddRoots(t *testing.T) {
	key, impl, cache := createRoots()
	if s := expectNotRecognized(impl, key, "alice"); s != "" {
		t.Error(s)
	}
	if s := expectNotRecognized(cache, key, "alice"); s != "" {
		t.Error(s)
	}

	if err := cache.Add(key, "alice:$"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := cache.Add(key, "bob"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if s := expectRecognized(impl, key, "alice"); s != "" {
		t.Error(s)
	}
	if s := expectRecognized(impl, key, "bob"); s != "" {
		t.Error(s)
	}
	if s := expectNotRecognized(impl, key, "alice:friend"); s != "" {
		t.Error(s)
	}
	if s := expectRecognized(impl, key, "bob:friend"); s != "" {
		t.Error(s)
	}
	if s := expectRecognized(cache, key, "alice"); s != "" {
		t.Error(s)
	}
	if s := expectRecognized(cache, key, "bob"); s != "" {
		t.Error(s)
	}
	if s := expectNotRecognized(cache, key, "alice:friend"); s != "" {
		t.Error(s)
	}
	if s := expectRecognized(cache, key, "bob:friend"); s != "" {
		t.Error(s)
	}

	if s := expectNotRecognized(impl, key, "carol"); s != "" {
		t.Error(s)
	}
	if s := expectNotRecognized(cache, key, "carol"); s != "" {
		t.Error(s)
	}
}

func TestNegativeCache(t *testing.T) {
	key, impl, cache := createRoots()

	if s := expectNotRecognized(cache, key, "alice"); s != "" {
		t.Error(s)
	}

	if err := impl.Add(key, "alice"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Should return the cached error.
	if s := expectNotRecognized(cache, key, "alice"); s != "" {
		t.Error(s)
	}

	// Until we flush...
	cache.flush()
	if s := expectRecognized(cache, key, "alice"); s != "" {
		t.Error(s)
	}
}

func TestRootsDebugString(t *testing.T) {
	key, impl, cache := createRoots()

	if err := impl.Add(key, "alice:friend"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if a, b := impl.DebugString(), cache.DebugString(); a != b {
		t.Errorf("DebugString doesn't match. Expected:\n%s\nGot:\n%s", a, b)
	}
}

func TestRootsDump(t *testing.T) {
	key, impl, cache := createRoots()

	if err := cache.Add(key, "alice:friend"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	orig := impl.Dump()
	if got := cache.Dump(); !reflect.DeepEqual(orig, got) {
		t.Errorf("Dump() got %v, want %v", got, orig)
	}

	impl.Add(key, "carol")
	if got := cache.Dump(); !reflect.DeepEqual(orig, got) {
		t.Errorf("Dump() got %v, want %v", got, orig)
	}

	cache.flush()
	if cur, got := impl.Dump(), cache.Dump(); !reflect.DeepEqual(cur, got) {
		t.Errorf("Dump() got %v, want %v", got, cur)
	}
}

func createStore(p security.Principal) (security.BlessingStore, *cachedStore) {
	var mu sync.RWMutex
	impl := p.BlessingStore()
	return impl, &cachedStore{mu: &mu, pubkey: p.PublicKey(), impl: impl}
}

func TestDefaultBlessing(t *testing.T) {
	p := testutil.NewPrincipal("bob")
	store, cache := createStore(p)

	bob, _ := store.Default()
	var notify <-chan struct{}
	var cached security.Blessings
	if cached, notify = cache.Default(); !reflect.DeepEqual(bob, cached) {
		t.Errorf("Default(): got: %v, want: %v", cached, bob)
	}

	alice, err := p.BlessSelf("alice")
	if err != nil {
		t.Fatalf("BlessSelf failed: %v", err)
	}
	err = store.SetDefault(alice)
	if err != nil {
		t.Fatalf("SetDefault failed: %v", err)
	}
	cache.flush()
	// This should trigger a notification (via closure of the channel)
	<-notify
	if cached, notify = cache.Default(); !reflect.DeepEqual(alice, cached) {
		t.Errorf("Default(): got: %v, want: %v", cached, alice)
	}

	carol, err := p.BlessSelf("carol")
	if err != nil {
		t.Fatalf("BlessSelf failed: %v", err)
	}
	err = cache.SetDefault(carol)
	if err != nil {
		t.Fatalf("SetDefault failed: %v", err)
	}
	<-notify

	if cur, _ := store.Default(); !reflect.DeepEqual(carol, cur) {
		t.Errorf("Default(): got: %v, want: %v", cur, carol)
	}
	if cached, notify = cache.Default(); !reflect.DeepEqual(carol, cached) {
		t.Errorf("Default(): got: %v, want: %v", cached, carol)
	}

	john, _ := testutil.NewPrincipal("john").BlessingStore().Default()
	if nil == cache.SetDefault(john) {
		t.Errorf("Expected error setting default with bad key.")
	}
	if cached, _ = cache.Default(); !reflect.DeepEqual(carol, cached) {
		t.Errorf("Default(): got: %v, want: %v", cached, carol)
	}

}

func TestSet(t *testing.T) {
	p := testutil.NewPrincipal("bob")
	store, cache := createStore(p)
	var noBlessings security.Blessings

	bob, _ := store.Default()
	alice, err := p.BlessSelf("alice")
	if err != nil {
		t.Fatalf("BlessSelf failed: %v", err)
	}
	john, _ := testutil.NewPrincipal("john").BlessingStore().Default()

	store.Set(noBlessings, "...")
	if _, err := cache.Set(bob, "bob"); err != nil {
		t.Errorf("Set() failed: %v", err)
	}

	if got := cache.ForPeer("bob:server"); !reflect.DeepEqual(bob, got) {
		t.Errorf("ForPeer(bob:server) got: %v, want: %v", got, bob)
	}

	blessings, err := cache.Set(noBlessings, "bob")
	if err != nil {
		t.Errorf("Set() failed: %v", err)
	}
	if !reflect.DeepEqual(bob, blessings) {
		t.Errorf("Previous blessings %v, wanted %v", blessings, bob)
	}
	if got, want := cache.ForPeer("bob:server"), (security.Blessings{}); !reflect.DeepEqual(want, got) {
		t.Errorf("ForPeer(bob:server) got: %v, want: %v", got, want)
	}

	blessings, err = cache.Set(john, "john")
	if err == nil {
		t.Errorf("No error from set")
	}
	if got := cache.ForPeer("john:server"); got.PublicKey() != nil {
		t.Errorf("ForPeer(john:server) got: %v, want: %v", got, nil)
	}

	blessings, err = cache.Set(bob, "...")
	if err != nil {
		t.Errorf("Set() failed: %v", err)
	}
	blessings, err = cache.Set(alice, "bob")
	if err != nil {
		t.Errorf("Set() failed: %v", err)
	}

	expected, err := security.UnionOfBlessings(bob, alice)
	if err != nil {
		t.Errorf("UnionOfBlessings failed: %v", err)
	}
	if got := cache.ForPeer("bob:server"); !reflect.DeepEqual(expected, got) {
		t.Errorf("ForPeer(bob:server) got: %v, want: %v", got, expected)
	}
}

func TestForPeerCaching(t *testing.T) {
	p := testutil.NewPrincipal("bob")
	store, cache := createStore(p)

	bob, _ := store.Default()
	alice, err := p.BlessSelf("alice")
	if err != nil {
		t.Fatalf("BlessSelf failed: %v", err)
	}

	store.Set(security.Blessings{}, "...")
	store.Set(bob, "bob")

	if got := cache.ForPeer("bob:server"); !reflect.DeepEqual(bob, got) {
		t.Errorf("ForPeer(bob:server) got: %v, want: %v", got, bob)
	}

	store.Set(alice, "bob")
	if got := cache.ForPeer("bob:server"); !reflect.DeepEqual(bob, got) {
		t.Errorf("ForPeer(bob:server) got: %v, want: %v", got, bob)
	}

	cache.flush()
	if got := cache.ForPeer("bob:server"); !reflect.DeepEqual(alice, got) {
		t.Errorf("ForPeer(bob:server) got: %v, want: %v", got, alice)
	}
}

func TestPeerBlessings(t *testing.T) {
	p := testutil.NewPrincipal("bob")
	store, cache := createStore(p)

	alice, err := p.BlessSelf("alice")
	if err != nil {
		t.Fatalf("BlessSelf failed: %v", err)
	}

	if _, err = cache.Set(alice, "alice"); err != nil {
		t.Errorf("Set() failed: %v", err)
	}

	orig := store.PeerBlessings()
	if got := cache.PeerBlessings(); !reflect.DeepEqual(orig, got) {
		t.Errorf("PeerBlessings() got %v, want %v", got, orig)
	}

	store.Set(alice, "carol")
	if got := cache.PeerBlessings(); !reflect.DeepEqual(orig, got) {
		t.Errorf("PeerBlessings() got %v, want %v", got, orig)
	}

	cache.flush()
	if cur, got := store.PeerBlessings(), cache.PeerBlessings(); !reflect.DeepEqual(cur, got) {
		t.Errorf("PeerBlessings() got %v, want %v", got, cur)
	}
}

func TestStoreDebugString(t *testing.T) {
	impl, cache := createStore(testutil.NewPrincipal("bob:friend:alice"))

	if a, b := impl.DebugString(), cache.DebugString(); a != b {
		t.Errorf("DebugString doesn't match. Expected:\n%s\nGot:\n%s", a, b)
	}
}
