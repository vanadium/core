// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"reflect"
	"testing"

	"v.io/v23/context"
)

func TestDischargeToAndFromWire(t *testing.T) {
	var (
		p   = newPrincipal(t)
		cav = newCaveat(NewPublicKeyCaveat(p.PublicKey(), "peoria", ThirdPartyRequirements{}, UnconstrainedUse()))
	)
	discharge, err := p.MintDischarge(cav, UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	// Should be able to round-trip from native type to a copy
	var d2 Discharge
	if err := roundTrip(discharge, &d2); err != nil || !reflect.DeepEqual(discharge, d2) {
		t.Errorf("Got (%#v, %v) want (%#v, nil)", d2, err, discharge)
	}
	// And from native type to a wire type
	var wire1, wire2 WireDischarge
	if err := roundTrip(discharge, &wire1); err != nil {
		t.Fatal(err)
	}
	// And from wire to another wire type
	if err := roundTrip(wire1, &wire2); err != nil || !reflect.DeepEqual(wire1, wire2) {
		t.Fatalf("Got (%#v, %v) want (%#v, nil)", wire2, err, wire1)
	}
	// And from wire to a native type
	var d3 Discharge
	if err := roundTrip(wire2, &d3); err != nil || !reflect.DeepEqual(discharge, d3) {
		t.Errorf("Got (%#v, %v) want (%#v, nil)", d3, err, discharge)
	}
}

func TestDischargeSignatureCaching(t *testing.T) {
	var (
		p1          = newPrincipal(t)
		p2          = newPrincipal(t)
		cav         = newCaveat(NewPublicKeyCaveat(p1.PublicKey(), "peoria", ThirdPartyRequirements{}, UnconstrainedUse()))
		ctx, cancel = context.RootContext()
		mkCall      = func(d Discharge) Call {
			return NewCall(&CallParams{
				RemoteDischarges: map[string]Discharge{cav.ThirdPartyDetails().ID(): d},
			})
		}
	)
	defer cancel()
	discharge1, err := p1.MintDischarge(cav, UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	// Another principal (p2) may mint a discharge for the same caveat, but it should not be accepted.
	discharge2, err := p2.MintDischarge(cav, UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	if err := cav.Validate(ctx, mkCall(discharge1)); err != nil {
		t.Error(err)
	}
	if err := cav.Validate(ctx, mkCall(discharge2)); err == nil {
		t.Errorf("Caveat that required a discharge from one principal was validated by a discharge created by another!")
	}
}

func BenchmarkDischargeEquality(b *testing.B) {
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	cav := newCaveat(NewPublicKeyCaveat(p.PublicKey(), "moline", ThirdPartyRequirements{}, UnconstrainedUse()))
	discharge, err := p.MintDischarge(cav, UnconstrainedUse())
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		discharge.Equivalent(discharge)
	}
}

func benchmarkPublicKeyDischargeVerification(caching bool, b *testing.B) {
	if !caching {
		dischargeSignatureCache.disable()
		defer dischargeSignatureCache.enable()
	}
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	cav := newCaveat(NewPublicKeyCaveat(p.PublicKey(), "moline", ThirdPartyRequirements{}, UnconstrainedUse()))
	discharge, err := p.MintDischarge(cav, UnconstrainedUse())
	if err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.RootContext()
	defer cancel()
	call := NewCall(&CallParams{
		RemoteDischarges: map[string]Discharge{
			cav.ThirdPartyDetails().ID(): discharge,
		}})
	// Validate once for the initial expensive signature validation (not stored in the cache).
	if err := cav.Validate(ctx, call); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cav.Validate(ctx, call); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublicKeyDischargeVerification_Cached(b *testing.B) {
	benchmarkPublicKeyDischargeVerification(true, b)
}
func BenchmarkPublicKeyDischargeVerification(b *testing.B) {
	benchmarkPublicKeyDischargeVerification(false, b)
}
