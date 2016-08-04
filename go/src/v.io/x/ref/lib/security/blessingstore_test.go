// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"testing"

	"v.io/v23/security"
)

type storeTester struct {
	forAll, forFoo, forBar, def security.Blessings
	other                       security.Blessings // Blessings bound to a different principal.
}

func (t *storeTester) testSet(s security.BlessingStore) error {
	testdata := []struct {
		blessings security.Blessings
		pattern   security.BlessingPattern
		wantErr   string
	}{
		{t.forAll, security.AllPrincipals, ""},
		{t.forAll, "$", ""},
		{t.forFoo, "foo", ""},
		{t.forBar, "bar:$", ""},
		{t.other, security.AllPrincipals, "public key does not match"},
		{t.forAll, "", "invalid BlessingPattern"},
		{t.forAll, "foo:$:bar", "invalid BlessingPattern"},
	}
	added := make(map[security.BlessingPattern]security.Blessings)
	for _, d := range testdata {
		_, err := s.Set(d.blessings, d.pattern)
		if merr := matchesError(err, d.wantErr); merr != nil {
			return fmt.Errorf("Set(%v, %q): %v", d.blessings, d.pattern, merr)
		}
		if err == nil {
			added[d.pattern] = d.blessings
		}
	}
	m := s.PeerBlessings()
	if !reflect.DeepEqual(added, m) {
		return fmt.Errorf("PeerBlessings(%v) != added(%v)", m, added)
	}
	return nil
}

func (t *storeTester) testSetDefault(s security.BlessingStore) error {
	if err := s.SetDefault(security.Blessings{}); err != nil {
		return fmt.Errorf("SetDefault({}): %v", err)
	}
	var notify <-chan struct{}
	var got security.Blessings
	if got, notify = s.Default(); !got.IsZero() {
		return fmt.Errorf("Default returned %v, wanted empty", got)
	}
	if err := s.SetDefault(t.def); err != nil {
		return fmt.Errorf("SetDefault(%v): %v", t.def, err)
	}
	<-notify
	if got, _ = s.Default(); !reflect.DeepEqual(got, t.def) {
		return fmt.Errorf("Default returned %v, want %v", got, t.def)
	}
	// Changing default to an invalid blessing should not affect the existing default.
	if err := matchesError(s.SetDefault(t.other), "public key does not match"); err != nil {
		return err
	}
	if got, _ := s.Default(); !reflect.DeepEqual(got, t.def) {
		return fmt.Errorf("Default returned %v, want %v", got, t.def)
	}
	return nil
}

func (t *storeTester) testForPeer(s security.BlessingStore) error {
	testdata := []struct {
		peers     []string
		blessings security.Blessings
	}{
		{nil, t.forAll},
		{[]string{"baz"}, t.forAll},
		{[]string{"foo"}, unionOfBlessings(t.forAll, t.forFoo)},
		{[]string{"bar"}, unionOfBlessings(t.forAll, t.forBar)},
		{[]string{"foo:foo"}, unionOfBlessings(t.forAll, t.forFoo)},
		{[]string{"bar:baz"}, t.forAll},
		{[]string{"foo:foo:bar"}, unionOfBlessings(t.forAll, t.forFoo)},
		{[]string{"bar:foo", "foo"}, unionOfBlessings(t.forAll, t.forFoo)},
		{[]string{"bar", "foo"}, unionOfBlessings(t.forAll, t.forFoo, t.forBar)},
	}
	for _, d := range testdata {
		if got, want := s.ForPeer(d.peers...), d.blessings; !reflect.DeepEqual(got, want) {
			return fmt.Errorf("ForPeer(%v): got: %v, want: %v", d.peers, got, want)
		}
	}
	return nil
}

func newStoreTester(blessed security.Principal) *storeTester {
	var (
		blessing = func(root, extension string) security.Blessings {
			blesser, err := NewPrincipal()
			if err != nil {
				panic(err)
			}
			blessing, err := blesser.Bless(blessed.PublicKey(), blessSelf(blesser, root), extension, security.UnconstrainedUse())
			if err != nil {
				panic(err)
			}
			return blessing
		}
	)
	pother, err := NewPrincipal()
	if err != nil {
		panic(err)
	}

	s := &storeTester{}
	s.forAll = blessing("bar", "alice")
	s.forFoo = blessing("foo", "alice")
	s.forBar = unionOfBlessings(s.forAll, s.forFoo)
	s.def = blessing("default", "alice")
	s.other = blessSelf(pother, "other")
	return s
}

func TestBlessingStore(t *testing.T) {
	p, err := NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	tester := newStoreTester(p)
	s := p.BlessingStore()
	if err := tester.testSet(s); err != nil {
		t.Error(err)
	}
	if err := tester.testForPeer(s); err != nil {
		t.Error(err)
	}
	if err := tester.testSetDefault(s); err != nil {
		t.Error(err)
	}
	testDischargeCache(t, s)
}

func TestBlessingStorePersistence(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestPersistingBlessingStore")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	p, err := CreatePersistentPrincipal(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	tester := newStoreTester(p)
	s := p.BlessingStore()

	if err := tester.testSet(s); err != nil {
		t.Error(err)
	}
	if err := tester.testForPeer(s); err != nil {
		t.Error(err)
	}
	if err := tester.testSetDefault(s); err != nil {
		t.Error(err)
	}
	testDischargeCache(t, s)

	// Recreate the BlessingStore from the directory.
	p2, err := LoadPersistentPrincipal(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	s = p2.BlessingStore()
	if err := tester.testForPeer(s); err != nil {
		t.Error(err)
	}
	if got, _ := s.Default(); !reflect.DeepEqual(got, tester.def) {
		t.Fatalf("Default(): got: %v, want: %v", got, tester.def)
	}
}

func TestBlessingStoreSetOverridesOldSetting(t *testing.T) {
	p, err := NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	var (
		alice = blessSelf(p, "alice")
		bob   = blessSelf(p, "bob")
		s     = p.BlessingStore()
		empty security.Blessings
	)
	// {alice, bob} is shared with "alice", whilst {bob} is shared with "alice:tv"
	if _, err := s.Set(alice, "alice:$"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Set(bob, "alice"); err != nil {
		t.Fatal(err)
	}
	if got, want := s.ForPeer("alice"), unionOfBlessings(alice, bob); !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
	if got, want := s.ForPeer("alice:friend"), bob; !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}

	// Clear out the blessing associated with "alice".
	// Now, bob should be shared with both alice and alice:friend.
	if _, err := s.Set(empty, "alice:$"); err != nil {
		t.Fatal(err)
	}
	if got, want := s.ForPeer("alice"), bob; !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
	if got, want := s.ForPeer("alice:friend"), bob; !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}

	// Clearing out an association that doesn't exist should have no effect.
	if _, err := s.Set(empty, "alice:enemy:$"); err != nil {
		t.Fatal(err)
	}
	if got, want := s.ForPeer("alice"), bob; !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
	if got, want := s.ForPeer("alice:friend"), bob; !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}

	// Clear everything
	if _, err := s.Set(empty, "alice"); err != nil {
		t.Fatal(err)
	}
	if got := s.ForPeer("alice"); !got.IsZero() {
		t.Errorf("Got %v, want empty", got)
	}
	if got := s.ForPeer("alice:friend"); !got.IsZero() {
		t.Errorf("Got %v, want empty", got)
	}
}

func TestBlessingStoreSetReturnsOldValue(t *testing.T) {
	p, err := NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	var (
		alice = blessSelf(p, "alice")
		bob   = blessSelf(p, "bob")
		s     = p.BlessingStore()
		empty security.Blessings
	)

	if old, err := s.Set(alice, security.AllPrincipals); !reflect.DeepEqual(old, empty) || err != nil {
		t.Errorf("Got (%v, %v), want (%v, nil)", old, err, empty)
	}
	if old, err := s.Set(alice, security.AllPrincipals); !reflect.DeepEqual(old, alice) || err != nil {
		t.Errorf("Got (%v, %v) want (%v, nil)", old, err, alice)
	}
	if old, err := s.Set(bob, security.AllPrincipals); !reflect.DeepEqual(old, alice) || err != nil {
		t.Errorf("Got (%v, %v) want (%v, nil)", old, err, alice)
	}
	if old, err := s.Set(empty, security.AllPrincipals); !reflect.DeepEqual(old, bob) || err != nil {
		t.Errorf("Got (%v, %v) want (%v, nil)", old, err, bob)
	}
}

func TestBlessingStoreDebugString(t *testing.T) {
	p, err := NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	var (
		alice = blessSelf(p, "alice")
		bob   = blessSelf(p, "bob")
		s     = p.BlessingStore()
	)
	if _, err := s.Set(alice, security.AllPrincipals); err != nil {
		t.Errorf("Got error %v", err)
	}
	if _, err := s.Set(bob, "-patternstartingwithdash"); err != nil {
		t.Errorf("Got error %v", err)
	}
	blessingPatternsRE := regexp.MustCompile("Peer pattern[^\n]*\n...[^\n]*\n-patternstartingwithdash")
	if ds := s.DebugString(); !blessingPatternsRE.MatchString(ds) {
		t.Errorf("DebugString (%s) doesn't match", ds)
	}

}
