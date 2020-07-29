// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/security"
)

func TestFixedBlessingStore(t *testing.T) {
	p, b1 := newPrincipal("self")
	b2, _ := p.BlessSelf("other")
	tpcav := mkCaveat(security.NewPublicKeyCaveat(p.PublicKey(), "location", security.ThirdPartyRequirements{}, security.UnconstrainedUse()))
	d := mkDischarge(p.MintDischarge(tpcav, security.UnconstrainedUse()))
	s1 := NewBlessingStore(p.PublicKey())
	s2 := FixedBlessingsStore(b1, s1)

	// Set and SetDefault should fail, other than that, s2 should behave
	// identically to s1 after setting s1 up
	s1.Set(b1, security.AllPrincipals) //nolint:errcheck
	s1.SetDefault(b1)                  //nolint:errcheck

	if _, err := s2.Set(b2, security.AllPrincipals); err == nil {
		t.Fatalf("%T.Set should fail", s2)
	}
	if err := s2.SetDefault(b2); err == nil {
		t.Fatalf("%T.SetDefault should fail", s2)
	}
	for idx, c := range []call{
		{"ForPeer", []interface{}{"foobar"}},
		{"PublicKey", nil},
		{"PeerBlessings", nil},
		{"CacheDischarge", []interface{}{d, tpcav, security.DischargeImpetus{}}},
		{"Discharge", []interface{}{tpcav, security.DischargeImpetus{}}},
		{"ClearDischarges", []interface{}{d}},
		{"Discharge", []interface{}{tpcav, security.DischargeImpetus{}}},
	} {
		if err := c.Compare(s1, s2); err != nil {
			t.Errorf("#%d: %v", idx, err)
		}
	}
	// Check call to Default separately since both the stores should return
	// the same blessing but possibly different channels.
	def1, _ := s1.Default()
	def2, _ := s2.Default()
	if !reflect.DeepEqual(def1, def2) {
		t.Errorf("Default: Got %v, want %v", def1, def2)
	}
}

func TestImmutableBlessingStore(t *testing.T) {
	p, b1 := newPrincipal("self")
	b2, _ := p.BlessSelf("othername")
	bdef, _ := p.BlessSelf("default")
	tpcav := mkCaveat(security.NewPublicKeyCaveat(p.PublicKey(), "location", security.ThirdPartyRequirements{}, security.UnconstrainedUse()))
	d := mkDischarge(p.MintDischarge(tpcav, security.UnconstrainedUse()))
	s1 := NewBlessingStore(p.PublicKey())
	s2 := ImmutableBlessingStore(s1)

	// Set and SetDefault called on s1 should affect s2
	if _, err := s1.Set(b1, security.AllPrincipals); err != nil {
		t.Fatal(err)
	}
	if err := s1.SetDefault(bdef); err != nil {
		t.Fatal(err)
	}
	// But Set and SetDefault on s2 should fail
	if _, err := s2.Set(b2, security.AllPrincipals); err == nil {
		t.Fatalf("%T.Set should fail", s2)
	}
	if err := s2.SetDefault(b2); err == nil {
		t.Fatalf("%T.SetDefault should fail", s2)
	}
	// All other method calls should defer to s1
	for idx, c := range []call{
		{"ForPeer", []interface{}{"foobar"}},
		{"Default", nil},
		{"PublicKey", nil},
		{"PeerBlessings", nil},
		{"CacheDischarge", []interface{}{d, tpcav, security.DischargeImpetus{}}},
		{"Discharge", []interface{}{tpcav, security.DischargeImpetus{}}},
		{"DebugString", nil},
		{"ClearDischarges", []interface{}{d}},
		{"Discharge", []interface{}{tpcav, security.DischargeImpetus{}}},
	} {
		if err := c.Compare(s1, s2); err != nil {
			t.Errorf("#%d: %v", idx, err)
		}
	}
}

func TestImmutableBlessingRoots(t *testing.T) {
	pk, _ := createAndMarshalPublicKey()
	r1 := NewBlessingRoots()
	r2 := ImmutableBlessingRoots(r1)
	pat := "pattern1"

	// Adding to r1 should affect r2
	if err := r1.Add(pk, security.BlessingPattern(pat)); err != nil {
		t.Fatal(err)
	}
	// But Add on r2 should fail
	if err := r2.Add(pk, "otherpattern"); err == nil {
		t.Errorf("%T.Add should fail", r2)
	}
	// All other methods should be the same
	for _, c := range []call{
		{"Recognized", []interface{}{pk, pat}},
		{"Dump", nil},
		{"DebugString", nil},
	} {
		if err := c.Compare(r1, r2); err != nil {
			t.Error(err)
		}
	}
}

type call struct {
	Method string
	Args   []interface{}
}

// Run executes c.Method on object with c.Args and returns the results.
func (c *call) Call(object interface{}) []interface{} {
	args := make([]reflect.Value, len(c.Args))
	for i, a := range c.Args {
		args[i] = reflect.ValueOf(a)
	}
	results := reflect.ValueOf(object).MethodByName(c.Method).Call(args)
	ret := make([]interface{}, len(results))
	for i, r := range results {
		ret[i] = r.Interface()
	}
	return ret
}

// Compare calls Run on o1 and o2 and returns an error if the results do not match.
func (c *call) Compare(o1, o2 interface{}) error {
	r1 := c.Call(o1)
	r2 := c.Call(o2)
	if !reflect.DeepEqual(r1, r2) {
		return fmt.Errorf("%v(%v): Got %v, want %v", c.Method, c.Args, r2, r1)
	}
	return nil
}
