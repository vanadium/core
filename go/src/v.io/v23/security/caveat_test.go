// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/uniqueid"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

func TestStandardCaveatFactories(t *testing.T) {
	var (
		self      = newPrincipal(t)
		balice, _ = UnionOfBlessings(blessSelf(t, self, "alice:phone"), blessSelf(t, self, "alice:tablet"))
		bother    = blessSelf(t, self, "other")
		b, _      = UnionOfBlessings(balice, bother)
		now       = time.Now()
		call      = NewCall(&CallParams{
			Timestamp:      now,
			Method:         "Foo",
			LocalPrincipal: self,
			LocalBlessings: b,
		})
		C     = newCaveat
		tests = []struct {
			cav Caveat
			ok  bool
		}{
			// ExpiryCaveat
			{C(NewExpiryCaveat(now.Add(time.Second))), true},
			{C(NewExpiryCaveat(now.Add(-1 * time.Second))), false},
			// MethodCaveat
			{C(NewMethodCaveat("Foo")), true},
			{C(NewMethodCaveat("Bar")), false},
			{C(NewMethodCaveat("Foo", "Bar")), true},
			{C(NewMethodCaveat("Bar", "Baz")), false},
			// PeerBlesingsCaveat
			{C(NewCaveat(PeerBlessingsCaveat, []BlessingPattern{"alice:phone"})), true},
			{C(NewCaveat(PeerBlessingsCaveat, []BlessingPattern{"alice:tablet", "other"})), true},
			{C(NewCaveat(PeerBlessingsCaveat, []BlessingPattern{"alice:laptop", "other", "alice:$"})), false},
			{C(NewCaveat(PeerBlessingsCaveat, []BlessingPattern{"alice:laptop", "other", "alice"})), true},
		}
	)

	AddToRoots(self, balice)

	ctx, cancel := context.RootContext()
	defer cancel()
	for idx, test := range tests {
		err := test.cav.Validate(ctx, call)
		if test.ok && err != nil {
			t.Errorf("#%d: %v.Validate(...) failed validation: %v", idx, test.cav, err)
		} else if !test.ok && verror.ErrorID(err) != ErrCaveatValidation.ID {
			t.Errorf("#%d: %v.Validate(...) returned error='%v' (errorid=%v), want errorid=%v", idx, test.cav, err, verror.ErrorID(err), ErrCaveatValidation.ID)
		}
	}
}

func TestPublicKeyThirdPartyCaveat(t *testing.T) {
	var (
		now              = time.Now()
		valid            = newCaveat(NewExpiryCaveat(now.Add(time.Second)))
		expired          = newCaveat(NewExpiryCaveat(now.Add(-1 * time.Second)))
		discharger       = newPrincipal(t)
		randomserver     = newPrincipal(t)
		ctxCancelAndCall = func(method string, discharges ...Discharge) (*context.T, context.CancelFunc, Call) {
			params := &CallParams{
				Timestamp:        now,
				Method:           method,
				RemoteDischarges: make(map[string]Discharge),
			}
			for _, d := range discharges {
				params.RemoteDischarges[d.ID()] = d
			}
			root, cancel := context.RootContext()
			return root, cancel, NewCall(params)
		}
	)

	tpc, err := NewPublicKeyCaveat(discharger.PublicKey(), "location", ThirdPartyRequirements{}, valid)
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
	d, err := discharger.MintDischarge(tpc, newCaveat(NewMethodCaveat("Method1")))
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
	var d2 Discharge
	if err := roundTrip(d, &d2); err != nil || !reflect.DeepEqual(d, d2) {
		t.Errorf("Got (%#v, %v), want (%#v, nil)", d2, err, d)
	}
	// A discharge minted by another principal should not be respected.
	if d, err = randomserver.MintDischarge(tpc, UnconstrainedUse()); err == nil {
		ctx3, cancel3, call3 := ctxCancelAndCall("Method1", d)
		defer cancel3()
		if err := matchesError(tpc.Validate(ctx3, call3), "signature verification on discharge"); err != nil {
			t.Fatal(err)
		}
	}
	// And ThirdPartyCaveat should not be dischargeable if caveats encoded within it fail validation.
	tpc, err = NewPublicKeyCaveat(discharger.PublicKey(), "location", ThirdPartyRequirements{}, expired)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.RootContext()
	defer cancel()
	call := NewCall(&CallParams{Timestamp: now})
	if merr := matchesError(tpc.ThirdPartyDetails().Dischargeable(ctx, call), "could not validate embedded restriction"); merr != nil {
		t.Fatal(merr)
	}
}

func TestCaveat(t *testing.T) {
	uid, err := uniqueid.Random()
	if err != nil {
		t.Fatal(err)
	}
	anyCd := CaveatDescriptor{
		Id:        uid,
		ParamType: vdl.AnyType,
	}
	if _, err := NewCaveat(anyCd, 9); verror.ErrorID(err) != ErrCaveatParamAny.ID {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), ErrCaveatParamAny.ID)
	}
	cd := CaveatDescriptor{
		Id:        uid,
		ParamType: vdl.TypeOf(string("")),
	}
	if _, err := NewCaveat(cd, nil); verror.ErrorID(err) != ErrCaveatParamTypeMismatch.ID {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), ErrCaveatParamTypeMismatch.ID)
	}
	// A param of type *vdl.Value with underlying type string should succeed.
	if _, err := NewCaveat(cd, vdl.StringValue(nil, "")); err != nil {
		t.Errorf("vdl value should have succeeded: %v", err)
	}
	call := NewCall(&CallParams{
		Method: "Foo",
	})
	ctx, cancel := context.RootContext()
	defer cancel()
	c1, err := NewCaveat(cd, "Foo")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := NewCaveat(cd, "Bar")
	if err != nil {
		t.Fatal(err)
	}
	// Validation will fail when the validation function isn't registered.
	if err := c1.Validate(ctx, call); verror.ErrorID(err) != ErrCaveatNotRegistered.ID {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), ErrCaveatNotRegistered.ID)
	}
	// Once registered, then it should be invoked
	RegisterCaveatValidator(cd, func(ctx *context.T, call Call, param string) error {
		if call.Method() == param {
			return nil
		}
		return fmt.Errorf("na na na")
	})
	if err := c1.Validate(ctx, call); err != nil {
		t.Error(err)
	}
	if err := c2.Validate(ctx, call); verror.ErrorID(err) != ErrCaveatValidation.ID {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), ErrCaveatValidation.ID)
	}
	// If a malformed caveat was received, then validation should fail
	c3 := Caveat{Id: cd.Id, ParamVom: nil}
	if err := c3.Validate(ctx, call); verror.ErrorID(err) != ErrCaveatParamCoding.ID {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), ErrCaveatParamCoding.ID)
	}
}

func TestRegisterCaveat(t *testing.T) {
	uid, err := uniqueid.Random()
	if err != nil {
		t.Fatal(err)
	}
	var (
		cd = CaveatDescriptor{
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
		RegisterCaveatValidator(cd, "not a function")
	}()
	func() {
		defer expectPanic("wrong #outputs")
		RegisterCaveatValidator(cd, func(*context.T, Call, string) (error, error) { return nil, nil })
	}()
	func() {
		defer expectPanic("bad output type")
		RegisterCaveatValidator(cd, func(*context.T, Call, string) int { return 0 })
	}()
	func() {
		defer expectPanic("wrong #inputs")
		RegisterCaveatValidator(cd, func(*context.T, string) error { return nil })
	}()
	func() {
		defer expectPanic("bad input arg 0")
		RegisterCaveatValidator(cd, func(int, Call, string) error { return nil })
	}()
	func() {
		defer expectPanic("bad input arg 1")
		RegisterCaveatValidator(cd, func(*context.T, int, string) error { return nil })
	}()
	func() {
		defer expectPanic("bad input arg 2")
		RegisterCaveatValidator(cd, func(*context.T, Call, int) error { return nil })
	}()
	func() {
		// Successful registration: No panic:
		RegisterCaveatValidator(cd, func(*context.T, Call, string) error { return nil })
	}()
	func() {
		defer expectPanic("Duplication registration")
		RegisterCaveatValidator(cd, func(*context.T, Call, string) error { return nil })
	}()
	if got, want := npanics, 8; got != want {
		t.Errorf("Got %d panics, want %d", got, want)
	}
}

func TestThirdPartyDetails(t *testing.T) {
	niltests := []Caveat{
		newCaveat(NewExpiryCaveat(time.Now())),
		newCaveat(NewMethodCaveat("Foo", "Bar")),
	}
	for _, c := range niltests {
		if tp := c.ThirdPartyDetails(); tp != nil {
			t.Errorf("Caveat [%v] recognized as a third-party caveat: %v", c, tp)
		}
	}
	req := ThirdPartyRequirements{ReportMethod: true}
	c, err := NewPublicKeyCaveat(newPrincipal(t).PublicKey(), "location", req, newCaveat(NewExpiryCaveat(time.Now())))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ThirdPartyDetails(); got.Location() != "location" {
		t.Errorf("Got location %q, want %q", got.Location(), "location")
	} else if !reflect.DeepEqual(got.Requirements(), req) {
		t.Errorf("Got requirements %+v, want %+v", got.Requirements(), req)
	}
}

func TestPublicKeyDischargeExpiry(t *testing.T) {
	var (
		discharger = newPrincipal(t)
		now        = time.Now()
		oneh       = newCaveat(NewExpiryCaveat(now.Add(time.Hour)))
		twoh       = newCaveat(NewExpiryCaveat(now.Add(2 * time.Hour)))
		threeh     = newCaveat(NewExpiryCaveat(now.Add(3 * time.Hour)))
	)

	tpc, err := NewPublicKeyCaveat(discharger.PublicKey(), "location", ThirdPartyRequirements{}, oneh)
	if err != nil {
		t.Fatal(err)
	}

	// Mint three discharges; one with no ExpiryCaveat...
	noExpiry, err := discharger.MintDischarge(tpc, newCaveat(NewMethodCaveat("Method1")))
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
		if _, err := NewExpiryCaveat(time.Time{}); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark caveat valdation using one of the simplest caveats (expiry).
func BenchmarkValidateCaveat(b *testing.B) {
	cav, err := NewExpiryCaveat(time.Now())
	if err != nil {
		b.Fatal(err)
	}
	call := NewCall(&CallParams{})
	ctx, cancel := context.RootContext()
	defer cancel()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cav.Validate(ctx, call)
	}
}
