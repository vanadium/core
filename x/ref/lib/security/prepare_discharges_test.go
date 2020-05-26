// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"fmt"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	securitylib "v.io/x/ref/lib/security"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

type expiryDischarger struct {
	durations map[string]time.Duration
}

func (ed *expiryDischarger) Discharge(ctx *context.T, call rpc.ServerCall, cav security.Caveat, _ security.DischargeImpetus) (security.Discharge, error) {
	tp := cav.ThirdPartyDetails()
	if tp == nil {
		return security.Discharge{}, fmt.Errorf("discharger: %v does not represent a third-party caveat", cav)
	}
	if err := tp.Dischargeable(ctx, call.Security()); err != nil {
		return security.Discharge{}, fmt.Errorf("third-party caveat %v cannot be discharged for this context: %v", cav, err)
	}
	duration := ed.durations[tp.ID()]
	if duration == 0 {
		duration = time.Minute
	}
	expiry, err := security.NewExpiryCaveat(time.Now().Add(duration))
	if err != nil {
		return security.Discharge{}, fmt.Errorf("failed to create an expiration on the discharge: %v", err)
	}
	d, err := call.Security().LocalPrincipal().MintDischarge(cav, expiry)
	if err != nil {
		return security.Discharge{}, err
	}
	ctx.Infof("got discharge on sever %#v", d)
	return d, nil
}

func inRange(v, start, end time.Time) error {
	if v.Before(start) || v.After(end) {
		return fmt.Errorf("Got %v, wanted value in (%v, %v)", v, start, end)
	}
	return nil
}

func TestPrepareDischarges(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	pclient := testutil.NewPrincipal("client")
	cctx, err := v23.WithPrincipal(ctx, pclient)
	if err != nil {
		t.Fatal(err)
	}
	pdischarger := testutil.NewPrincipal("discharger")
	dctx, err := v23.WithPrincipal(ctx, pdischarger)
	if err != nil {
		t.Fatal(err)
	}
	defaultBlessings := func(p security.Principal) security.Blessings {
		d, _ := p.BlessingStore().Default()
		return d
	}
	err = security.AddToRoots(pclient, defaultBlessings(pdischarger))
	if err != nil {
		t.Fatal(err)
	}
	err = security.AddToRoots(pclient, defaultBlessings(v23.GetPrincipal(ctx)))
	if err != nil {
		t.Fatal(err)
	}
	err = security.AddToRoots(pdischarger, defaultBlessings(pclient))
	if err != nil {
		t.Fatal(err)
	}
	err = security.AddToRoots(pdischarger, defaultBlessings(v23.GetPrincipal(ctx)))
	if err != nil {
		t.Fatal(err)
	}

	expcav, err := security.NewExpiryCaveat(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	expiryDur := 100 * time.Millisecond
	tpcav, err := security.NewPublicKeyCaveat(
		pdischarger.PublicKey(),
		"discharger",
		security.ThirdPartyRequirements{},
		expcav)
	if err != nil {
		t.Fatal(err)
	}
	tpcavLong, err := security.NewPublicKeyCaveat(
		pdischarger.PublicKey(),
		"discharger",
		security.ThirdPartyRequirements{},
		expcav)
	if err != nil {
		t.Fatal(err)
	}
	cbless, err := pclient.BlessSelf("clientcaveats", tpcav, tpcavLong)
	if err != nil {
		t.Fatal(err)
	}
	tpid := tpcav.ThirdPartyDetails().ID()

	_, _, err = v23.WithNewServer(dctx,
		"discharger",
		&expiryDischarger{map[string]time.Duration{
			tpcav.ThirdPartyDetails().ID(): 100 * time.Millisecond,
		}},
		security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}

	// Fetch discharges for tpcav.
	beforeFetch := time.Now()
	discharges, refreshTime := securitylib.PrepareDischarges(cctx, cbless,
		nil, "", nil)
	afterFetch := time.Now()
	if len(discharges) != 2 {
		t.Errorf("Got %d discharges, expected 2.", len(discharges))
	}
	dis, has := discharges[tpid]
	if !has {
		t.Errorf("Got %#v, Expected discharge for %s", discharges, tpid)
	}
	// The refreshTime should be expiryDur/2 ms after the fetch, since that's half the
	// lifetime of the discharge.
	if err := inRange(refreshTime, beforeFetch.Add(expiryDur/2), afterFetch.Add(expiryDur/2)); err != nil {
		t.Error(err)
	}
	if err := inRange(dis.Expiry(), beforeFetch.Add(expiryDur), afterFetch.Add(expiryDur)); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Until(dis.Expiry()))

	// Preparing Discharges again to get fresh discharges.
	beforeFetch = time.Now()
	discharges, refreshTime = securitylib.PrepareDischarges(cctx, cbless,
		nil, "", nil)
	afterFetch = time.Now()
	if len(discharges) != 2 {
		t.Errorf("Got %d discharges, expected 2.", len(discharges))
	}
	dis, has = discharges[tpid]
	if !has {
		t.Errorf("Got %#v, Expected discharge for %s", discharges, tpid)
	}
	if err := inRange(refreshTime, beforeFetch.Add(expiryDur/2), afterFetch.Add(expiryDur/2)); err != nil {
		t.Error(err)
	}
	if err := inRange(dis.Expiry(), beforeFetch.Add(expiryDur), afterFetch.Add(expiryDur)); err != nil {
		t.Error(err)
	}
}
