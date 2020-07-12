// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
)

func TestDischargeToAndFromWireECDSA(t *testing.T) {
	testDischargeToAndFromWire(t, sectest.NewECDSAPrincipalP256(t))
}

func TestDischargeToAndFromWireED25519(t *testing.T) {
	testDischargeToAndFromWire(t, sectest.NewED25519Principal(t))
}

func testDischargeToAndFromWire(t *testing.T, p security.Principal) {
	cav := sectest.NewPublicKeyUnconstrainedCaveat(t, p, "peoria")
	discharge, err := p.MintDischarge(cav, security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	// Should be able to round-trip from native type to a copy
	var d2 security.Discharge
	if err := sectest.RoundTrip(discharge, &d2); err != nil || !reflect.DeepEqual(discharge, d2) {
		t.Errorf("Got (%#v, %v) want (%#v, nil)", d2, err, discharge)
	}
	// And from native type to a wire type
	var wire1, wire2 security.WireDischarge
	if err := sectest.RoundTrip(discharge, &wire1); err != nil {
		t.Fatal(err)
	}
	// And from wire to another wire type
	if err := sectest.RoundTrip(wire1, &wire2); err != nil || !reflect.DeepEqual(wire1, wire2) {
		t.Fatalf("Got (%#v, %v) want (%#v, nil)", wire2, err, wire1)
	}
	// And from wire to a native type
	var d3 security.Discharge
	if err := sectest.RoundTrip(wire2, &d3); err != nil || !reflect.DeepEqual(discharge, d3) {
		t.Errorf("Got (%#v, %v) want (%#v, nil)", d3, err, discharge)
	}
}
func TestDischargeSignatureCachingECDSA(t *testing.T) {
	testDischargeSignatureCaching(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func TestDischargeSignatureCachingED25519(t *testing.T) {
	testDischargeSignatureCaching(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t))
}

func TestDischargeSignatureCaching(t *testing.T) {
	testDischargeSignatureCaching(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t))
	testDischargeSignatureCaching(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t))
}

func testDischargeSignatureCaching(t *testing.T, p1, p2 security.Principal) {
	var (
		cav         = sectest.NewPublicKeyUnconstrainedCaveat(t, p1, "peoria")
		ctx, cancel = context.RootContext()
		mkCall      = func(d security.Discharge) security.Call {
			return security.NewCall(&security.CallParams{
				RemoteDischarges: map[string]security.Discharge{cav.ThirdPartyDetails().ID(): d},
			})
		}
	)
	defer cancel()
	discharge1, err := p1.MintDischarge(cav, security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	// Another principal (p2) may mint a discharge for the same caveat, but it should not be accepted.
	discharge2, err := p2.MintDischarge(cav, security.UnconstrainedUse())
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

func BenchmarkDischargeEqualityECDSA(b *testing.B) {
	benchmarkDischargeEquality(b, sectest.NewECDSASignerP256(b))

}

func BenchmarkDischargeEqualityED25519(b *testing.B) {
	benchmarkDischargeEquality(b, sectest.NewED25519Signer(b))
}

func benchmarkDischargeEquality(b *testing.B, signer security.Signer) {
	p, err := security.CreatePrincipal(signer, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	cav := sectest.NewPublicKeyUnconstrainedCaveat(b, p, "moline")
	discharge, err := p.MintDischarge(cav, security.UnconstrainedUse())
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		discharge.Equivalent(discharge)
	}
}

func benchmarkPublicKeyDischargeVerification(caching bool, b *testing.B, signer security.Signer) {
	if !caching {
		security.DisableDischargesSignatureCache()
		defer security.EnableDischargesSignatureCache()
	}
	p, err := security.CreatePrincipal(signer, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	cav := sectest.NewPublicKeyUnconstrainedCaveat(b, p, "moline")
	discharge, err := p.MintDischarge(cav, security.UnconstrainedUse())
	if err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.RootContext()
	defer cancel()
	call := security.NewCall(&security.CallParams{
		RemoteDischarges: map[string]security.Discharge{
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

func BenchmarkPublicKeyDischargeVerificationCachedECDSA(b *testing.B) {
	benchmarkPublicKeyDischargeVerification(true, b, sectest.NewECDSASignerP256(b))
}

func BenchmarkPublicKeyDischargeVerificationCachedED25519(b *testing.B) {
	benchmarkPublicKeyDischargeVerification(true, b, sectest.NewED25519Signer(b))
}

func BenchmarkPublicKeyDischargeVerificationECDSA(b *testing.B) {
	benchmarkPublicKeyDischargeVerification(false, b, sectest.NewECDSASignerP256(b))
}

func BenchmarkPublicKeyDischargeVerificationED25519(b *testing.B) {
	benchmarkPublicKeyDischargeVerification(false, b, sectest.NewED25519Signer(b))
}
