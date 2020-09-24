// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/v23/uniqueid"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

func TestStandardCaveatFactoriesECDSA(t *testing.T) {
	testStandardCaveatFactories(t, sectest.NewECDSAPrincipalP256(t))
}

func TestStandardCaveatFactoriesED25519(t *testing.T) {
	testStandardCaveatFactories(t, sectest.NewED25519Principal(t))
}

func testStandardCaveatFactories(t *testing.T, self security.Principal) {
	var (
		balice, _ = security.UnionOfBlessings(
			sectest.BlessSelf(t, self, "alice:phone"),
			sectest.BlessSelf(t, self, "alice:tablet"))
		bother = sectest.BlessSelf(t, self, "other")
		b, _   = security.UnionOfBlessings(balice, bother)
		now    = time.Now()
		call   = security.NewCall(&security.CallParams{
			Timestamp:      now,
			Method:         "Foo",
			LocalPrincipal: self,
			LocalBlessings: b,
		})
		C = func(c security.Caveat, err error) security.Caveat {
			if err != nil {
				t.Fatal(err)
			}
			return c
		}
		tests = []struct {
			cav security.Caveat
			ok  bool
		}{
			// ExpiryCaveat
			{C(security.NewExpiryCaveat(now.Add(time.Second))), true},
			{C(security.NewExpiryCaveat(now.Add(-1 * time.Second))), false},
			// MethodCaveat
			{C(security.NewMethodCaveat("Foo")), true},
			{C(security.NewMethodCaveat("Bar")), false},
			{C(security.NewMethodCaveat("Foo", "Bar")), true},
			{C(security.NewMethodCaveat("Bar", "Baz")), false},
			// PeerBlesingsCaveat
			{C(security.NewCaveat(security.PeerBlessingsCaveat,
				[]security.BlessingPattern{"alice:phone"})),
				true},
			{C(security.NewCaveat(security.PeerBlessingsCaveat,
				[]security.BlessingPattern{"alice:tablet", "other"})),
				true},
			{C(security.NewCaveat(security.PeerBlessingsCaveat,
				[]security.BlessingPattern{"alice:laptop", "other", "alice:$"})),
				false},
			{C(security.NewCaveat(security.PeerBlessingsCaveat,
				[]security.BlessingPattern{"alice:laptop", "other", "alice"})),
				true},
		}
	)

	if err := security.AddToRoots(self, balice); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.RootContext()
	defer cancel()
	for idx, test := range tests {
		err := test.cav.Validate(ctx, call)
		if test.ok && err != nil {
			t.Errorf("#%d: %v.Validate(...) failed validation: %v", idx, test.cav, err)
		} else if !test.ok && !errors.Is(err, security.ErrCaveatValidation) {
			t.Errorf("#%d: %v.Validate(...) returned error='%v' (errorid=%v), want errorid=%v", idx, test.cav, err, verror.ErrorID(err), security.ErrCaveatValidation.ID)
		}
	}
}
func TestPublicKeyThirdPartyCaveatECDSA(t *testing.T) {
	testPublicKeyThirdPartyCaveat(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func TestPublicKeyThirdPartyCaveatED25519(t *testing.T) {
	testPublicKeyThirdPartyCaveat(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestPublicKeyThirdPartyCaveat(t *testing.T) {
	testPublicKeyThirdPartyCaveat(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
	)
	testPublicKeyThirdPartyCaveat(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func testPublicKeyThirdPartyCaveat(t *testing.T, discharger,
	randomserver security.Principal) {
	var (
		now     = time.Now()
		valid   = sectest.NewExpiryCaveat(t, now.Add(time.Second))
		expired = sectest.NewExpiryCaveat(t, now.Add(-1*time.Second))

		ctxCancelAndCall = func(method string, discharges ...security.Discharge) (*context.T, context.CancelFunc, security.Call) {
			params := &security.CallParams{
				Timestamp:        now,
				Method:           method,
				RemoteDischarges: make(map[string]security.Discharge),
			}
			for _, d := range discharges {
				params.RemoteDischarges[d.ID()] = d
			}
			root, cancel := context.RootContext()
			return root, cancel, security.NewCall(params)
		}
	)

	tpc, err := security.NewPublicKeyCaveat(discharger.PublicKey(), "location", security.ThirdPartyRequirements{}, valid)
	if err != nil {
		t.Fatal(err)
	}
	// Caveat should fail validation without a discharge
	ctx0, cancel0, call0 := ctxCancelAndCall("Method1")
	defer cancel0()
	if err := matchesError(tpc.Validate(ctx0, call0), "missing discharge"); err != nil {
		t.Fatal(err)
	}
	// Should validate when the discharge is present (and caveats on the discharge are met).
	d, err := discharger.MintDischarge(tpc, sectest.NewMethodCaveat(t, "Method1"))
	if err != nil {
		t.Fatal(err)
	}
	ctx1, cancel1, call1 := ctxCancelAndCall("Method1", d)
	defer cancel1()
	if err := tpc.Validate(ctx1, call1); err != nil {
		t.Fatal(err)
	}
	// Should fail validation when caveats on the discharge are not met.
	ctx2, cancel2, call2 := ctxCancelAndCall("Method2", d)
	defer cancel2()
	if err := matchesError(tpc.Validate(ctx2, call2), "discharge failed to validate"); err != nil {
		t.Fatal(err)
	}
	// Discharge can be converted to and from wire format:
	var d2 security.Discharge
	if err := sectest.RoundTrip(d, &d2); err != nil || !reflect.DeepEqual(d, d2) {
		t.Errorf("Got (%#v, %v), want (%#v, nil)", d2, err, d)
	}
	// A discharge minted by another principal should not be respected.
	if d, err = randomserver.MintDischarge(tpc, security.UnconstrainedUse()); err == nil {
		ctx3, cancel3, call3 := ctxCancelAndCall("Method1", d)
		defer cancel3()
		if err := matchesError(tpc.Validate(ctx3, call3), "signature verification on discharge"); err != nil {
			t.Fatal(err)
		}
	}
	// And ThirdPartyCaveat should not be dischargeable if caveats encoded within it fail validation.
	tpc, err = security.NewPublicKeyCaveat(discharger.PublicKey(), "location", security.ThirdPartyRequirements{}, expired)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.RootContext()
	defer cancel()
	call := security.NewCall(&security.CallParams{Timestamp: now})
	if merr := matchesError(tpc.ThirdPartyDetails().Dischargeable(ctx, call), "could not validate embedded restriction"); merr != nil {
		t.Fatal(merr)
	}
}

func TestCaveat(t *testing.T) {
	uid, err := uniqueid.Random()
	if err != nil {
		t.Fatal(err)
	}
	anyCd := security.CaveatDescriptor{
		Id:        uid,
		ParamType: vdl.AnyType,
	}
	if _, err := security.NewCaveat(anyCd, 9); !errors.Is(err, security.ErrCaveatParamAny) {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), security.ErrCaveatParamAny.ID)
	}
	cd := security.CaveatDescriptor{
		Id:        uid,
		ParamType: vdl.TypeOf(string("")),
	}
	if _, err := security.NewCaveat(cd, nil); !errors.Is(err, security.ErrCaveatParamTypeMismatch) {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), security.ErrCaveatParamTypeMismatch.ID)
	}
	// A param of type *vdl.Value with underlying type string should succeed.
	if _, err := security.NewCaveat(cd, vdl.StringValue(nil, "")); err != nil {
		t.Errorf("vdl value should have succeeded: %v", err)
	}
	call := security.NewCall(&security.CallParams{
		Method: "Foo",
	})
	ctx, cancel := context.RootContext()
	defer cancel()
	c1, err := security.NewCaveat(cd, "Foo")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := security.NewCaveat(cd, "Bar")
	if err != nil {
		t.Fatal(err)
	}
	// Validation will fail when the validation function isn't registered.
	if err := c1.Validate(ctx, call); !errors.Is(err, security.ErrCaveatNotRegistered) {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), security.ErrCaveatNotRegistered.ID)
	}
	// Once registered, then it should be invoked
	security.RegisterCaveatValidator(cd, func(ctx *context.T, call security.Call, param string) error {
		if call.Method() == param {
			return nil
		}
		return fmt.Errorf("na na na")
	})
	if err := c1.Validate(ctx, call); err != nil {
		t.Error(err)
	}
	if err := c2.Validate(ctx, call); !errors.Is(err, security.ErrCaveatValidation) {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), security.ErrCaveatValidation.ID)
	}
	// If a malformed caveat was received, then validation should fail
	c3 := security.Caveat{Id: cd.Id, ParamVom: nil}
	if err := c3.Validate(ctx, call); !errors.Is(err, security.ErrCaveatParamCoding) {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), security.ErrCaveatParamCoding.ID)
	}
}

func TestRegisterCaveat(t *testing.T) {
	uid, err := uniqueid.Random()
	if err != nil {
		t.Fatal(err)
	}
	var (
		cd = security.CaveatDescriptor{
			Id:        uid,
			ParamType: vdl.TypeOf(string("")),
		}
		npanics     int
		expectPanic = func(details string) {
			npanics++
			if err := recover(); err == nil {
				t.Errorf("%s: expected a panic", details)
			}
		}
	)
	func() {
		defer expectPanic("not a function")
		security.RegisterCaveatValidator(cd, "not a function")
	}()
	func() {
		defer expectPanic("wrong #outputs")
		security.RegisterCaveatValidator(cd, func(*context.T, security.Call, string) (error, error) { return nil, nil })
	}()
	func() {
		defer expectPanic("bad output type")
		security.RegisterCaveatValidator(cd, func(*context.T, security.Call, string) int { return 0 })
	}()
	func() {
		defer expectPanic("wrong #inputs")
		security.RegisterCaveatValidator(cd, func(*context.T, string) error { return nil })
	}()
	func() {
		defer expectPanic("bad input arg 0")
		security.RegisterCaveatValidator(cd, func(int, security.Call, string) error { return nil })
	}()
	func() {
		defer expectPanic("bad input arg 1")
		security.RegisterCaveatValidator(cd, func(*context.T, int, string) error { return nil })
	}()
	func() {
		defer expectPanic("bad input arg 2")
		security.RegisterCaveatValidator(cd, func(*context.T, security.Call, int) error { return nil })
	}()
	func() {
		// Successful registration: No panic:
		security.RegisterCaveatValidator(cd, func(*context.T, security.Call, string) error { return nil })
	}()
	func() {
		defer expectPanic("Duplication registration")
		security.RegisterCaveatValidator(cd, func(*context.T, security.Call, string) error { return nil })
	}()
	if got, want := npanics, 8; got != want {
		t.Errorf("Got %d panics, want %d", got, want)
	}
}

func TestThirdPartyDetailsECDSA(t *testing.T) {
	testThirdPartyDetails(t, sectest.NewECDSAPrincipalP256(t))
}

func TestThirdPartyDetailsED25519(t *testing.T) {
	testThirdPartyDetails(t, sectest.NewED25519Principal(t))
}

func testThirdPartyDetails(t *testing.T, p security.Principal) {
	niltests := []security.Caveat{
		sectest.NewExpiryCaveat(t, time.Now()),
		sectest.NewMethodCaveat(t, "Foo", "Bar"),
	}
	for _, c := range niltests {
		if tp := c.ThirdPartyDetails(); tp != nil {
			t.Errorf("Caveat [%v] recognized as a third-party caveat: %v", c, tp)
		}
	}
	req := security.ThirdPartyRequirements{ReportMethod: true}
	c, err := security.NewPublicKeyCaveat(p.PublicKey(), "location", req, sectest.NewExpiryCaveat(t, time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ThirdPartyDetails(); got.Location() != "location" {
		t.Errorf("Got location %q, want %q", got.Location(), "location")
	} else if !reflect.DeepEqual(got.Requirements(), req) {
		t.Errorf("Got requirements %+v, want %+v", got.Requirements(), req)
	}
}

func TestPublicKeyDischargeExpiryECDSA(t *testing.T) {
	testPublicKeyDischargeExpiry(t, sectest.NewECDSAPrincipalP256(t))
}

func TestPublicKeyDischargeExpiryED25519(t *testing.T) {
	testPublicKeyDischargeExpiry(t, sectest.NewED25519Principal(t))
}

func testPublicKeyDischargeExpiry(t *testing.T, discharger security.Principal) {
	var (
		now    = time.Now()
		oneh   = sectest.NewExpiryCaveat(t, now.Add(time.Hour))
		twoh   = sectest.NewExpiryCaveat(t, now.Add(2*time.Hour))
		threeh = sectest.NewExpiryCaveat(t, now.Add(3*time.Hour))
	)

	tpc, err := security.NewPublicKeyCaveat(discharger.PublicKey(), "location", security.ThirdPartyRequirements{}, oneh)
	if err != nil {
		t.Fatal(err)
	}

	// Mint three discharges; one with no ExpiryCaveat...
	noExpiry, err := discharger.MintDischarge(tpc, sectest.NewMethodCaveat(t, "Method1"))
	if err != nil {
		t.Fatal(err)
	}
	// another with an ExpiryCaveat one hour from now...
	oneCav, err := discharger.MintDischarge(tpc, oneh)
	if err != nil {
		t.Fatal(err)
	}
	// and finally, one with an ExpiryCaveat of one, two, and three hours from now.
	// Use a random order to help test that Expiry always returns the earliest time.
	threeCav, err := discharger.MintDischarge(tpc, threeh, oneh, twoh)
	if err != nil {
		t.Fatal(err)
	}

	if exp := noExpiry.Expiry(); !exp.IsZero() {
		t.Errorf("got %v, want %v", exp, time.Time{})
	}
	if got, want := oneCav.Expiry().UTC(), now.Add(time.Hour).UTC(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := threeCav.Expiry().UTC(), now.Add(time.Hour).UTC(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Benchmark creation of a new caveat using one of the simplest caveats
// (expiry)
func BenchmarkNewCaveat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := security.NewExpiryCaveat(time.Time{}); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark caveat valdation using one of the simplest caveats (expiry).
func BenchmarkValidateCaveat(b *testing.B) {
	cav, err := security.NewExpiryCaveat(time.Now().Add(time.Hour))
	if err != nil {
		b.Fatal(err)
	}
	call := security.NewCall(&security.CallParams{})
	ctx, cancel := context.RootContext()
	defer cancel()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cav.Validate(ctx, call); err != nil {
			b.Fatalf("iteration: %v: %v", i, err)
		}
	}
}
