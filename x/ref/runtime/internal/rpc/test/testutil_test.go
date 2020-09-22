// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/uniqueid"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security/bcrypter"
	"v.io/x/ref/test/testutil"
)

var errMethod = verror.ErrAborted.Errorf(nil, "aborted")
var fakeTimeCaveat = security.CaveatDescriptor{
	Id:        uniqueid.Id{0x18, 0xba, 0x6f, 0x84, 0xd5, 0xec, 0xdb, 0x9b, 0xf2, 0x32, 0x19, 0x5b, 0x53, 0x92, 0x80, 0x0},
	ParamType: vdl.TypeOf(int64(0)),
}

func init() {
	security.RegisterCaveatValidator(fakeTimeCaveat, func(_ *context.T, _ security.Call, t int64) error {
		if now := clock.Now(); now > t {
			return fmt.Errorf("fakeTimeCaveat expired: now=%d > then=%d", now, t)
		}
		return nil
	})
}

var clock = new(fakeClock)

type fakeClock struct {
	sync.Mutex
	time int64
}

func (c *fakeClock) Now() int64 {
	c.Lock()
	defer c.Unlock()
	return c.time
}

func (c *fakeClock) Advance(steps uint) {
	c.Lock()
	c.time += int64(steps)
	c.Unlock()
}

// singleBlessingStore implements security.BlessingStore. It is a
// BlessingStore that marks the last blessing that was set on it as
// shareable with any peer. It does not care about the public key that
// blessing being set is bound to.
type singleBlessingStore struct {
	pk  security.PublicKey
	b   security.Blessings
	def security.Blessings
}

func (s *singleBlessingStore) Set(b security.Blessings, _ security.BlessingPattern) (security.Blessings, error) {
	s.b = b
	return security.Blessings{}, nil
}
func (s *singleBlessingStore) ForPeer(...string) security.Blessings {
	return s.b
}
func (s *singleBlessingStore) SetDefault(b security.Blessings) error {
	s.def = b
	return nil
}
func (s *singleBlessingStore) Default() (security.Blessings, <-chan struct{}) {
	return s.def, nil
}
func (s *singleBlessingStore) PublicKey() security.PublicKey {
	return s.pk // This may be inconsistent with s.b & s.def, by design, for tests.
}
func (*singleBlessingStore) DebugString() string {
	return ""
}
func (*singleBlessingStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	return nil
}
func (*singleBlessingStore) CacheDischarge(security.Discharge, security.Caveat, security.DischargeImpetus) error {
	return nil
}
func (*singleBlessingStore) ClearDischarges(...security.Discharge) {
}
func (*singleBlessingStore) Discharge(security.Caveat, security.DischargeImpetus) (security.Discharge, time.Time) {
	return security.Discharge{}, time.Time{}
}

func extractKey(t *testing.T, rootCtx *context.T, root *bcrypter.Root, blessing string) *bcrypter.PrivateKey {
	key, err := root.Extract(rootCtx, blessing)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func withPrincipal(t *testing.T, ctx *context.T, name string, caveats ...security.Caveat) *context.T {
	idp := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
	p := testutil.NewPrincipal()
	if err := idp.Bless(p, name, caveats...); err != nil {
		t.Fatal(err)
	}
	ctx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func bless(t *testing.T, pctx, whoctx *context.T, extension string, cavs ...security.Caveat) security.Blessings {
	idp := testutil.IDProviderFromPrincipal(v23.GetPrincipal(pctx))
	b, err := idp.NewBlessings(v23.GetPrincipal(whoctx), extension, cavs...)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mkCaveat(cav security.Caveat, err error) security.Caveat {
	if err != nil {
		panic(err)
	}
	return cav
}

func mkThirdPartyCaveat(discharger security.PublicKey, location string, c security.Caveat) security.Caveat {
	tpc, err := security.NewPublicKeyCaveat(discharger, location, security.ThirdPartyRequirements{}, c)
	if err != nil {
		panic(err)
	}
	return tpc
}

func matchesErrorPattern(err error, id verror.IDAction, pattern string) bool {
	if len(pattern) > 0 && err != nil && !strings.Contains(err.Error(), pattern) {
		fmt.Printf("\n\n %v not in %v\n\n\n", pattern, err.Error())
		return false
	}
	if err == nil && id.ID == "" {
		return true
	}
	return errors.Is(err, id)
}

func waitForNames(t *testing.T, ctx *context.T, exist bool, names ...string) {
	for _, n := range names {
		for {
			me, err := v23.GetNamespace(ctx).Resolve(ctx, n)
			if err == nil && exist && len(me.Names()) > 0 {
				break
			}
			if (err != nil && !exist) || (err == nil && len(me.Names()) == 0) {
				break
			}
			ctx.Infof("Still waiting for %v, %v: %#v, %v", exist, n, me, err)
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func makeResultPtrs(ins []interface{}) []interface{} {
	outs := make([]interface{}, len(ins))
	for ix, in := range ins {
		typ := reflect.TypeOf(in)
		if typ == nil {
			// Nil indicates interface{}.
			var empty interface{}
			typ = reflect.ValueOf(&empty).Elem().Type()
		}
		outs[ix] = reflect.New(typ).Interface()
	}
	return outs
}

func checkResultPtrs(t *testing.T, name string, gotptrs, want []interface{}) {
	for ix, res := range gotptrs {
		got := reflect.ValueOf(res).Elem().Interface()
		want := want[ix]
		switch g := got.(type) {
		case verror.E:
			w, ok := want.(verror.E)
			// don't use reflect deep equal on verror's since they contain
			// a list of stack PCs which will be different.
			if !ok {
				t.Errorf("%s result %d got type %T, want %T", name, ix, g, w)
			}
			if verror.ErrorID(g) != w.ID {
				t.Errorf("%s result %d got %v, want %v", name, ix, g, w)
			}
		default:
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s result %d got %v, want %v", name, ix, got, want)
			}
		}
	}
}
