// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"bytes"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/v23/uniqueid"
	"v.io/v23/vdl"
	"v.io/v23/vom"
)

func roundTrip(t *testing.T, wireError error) error {
	buf, err := vom.Encode(wireError)
	if err != nil {
		_, _, line, _ := runtime.Caller(1)
		t.Fatalf("line: %v: wire error %v: %v", line, wireError, err)
	}
	var rxErr error
	if err := vom.Decode(buf, &rxErr); err != nil {
		_, _, line, _ := runtime.Caller(1)
		t.Fatalf("line: %v: wire error %v: %v", line, wireError, err)
	}
	return rxErr
}

func TestBasicErrors(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()

	err := security.ErrorfUnrecognizedRoot(ctx, "%v: %v", "bad-root", fmt.Errorf("unrecognised root"))
	err = roundTrip(t, err)
	_, _, root, details, _ := security.ParamsErrUnrecognizedRoot(err)
	if got, want := root, "bad-root"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := details.Error(), "unrecognised root"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	err = security.ErrorfAuthorizationFailed(ctx, "blessings %v: rejected %v: %v",
		[]string{"a", "b"},
		[]security.RejectedBlessing{},
		[]string{"localA", "localB"},
	)
	err = roundTrip(t, err)
	_, _, remote, _, _, _ := security.ParamsErrAuthorizationFailed(err)
	if got, want := remote, []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	origID, err := uniqueid.Random()
	if err != nil {
		t.Fatal(err)
	}
	err = security.ErrorfInvalidSigningBlessingCaveat(ctx, "id: %v", origID)
	err = roundTrip(t, err)
	_, _, id, _ := security.ParamsErrInvalidSigningBlessingCaveat(err)
	if got, want := id, origID; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	err = security.ErrorfPublicKeyNotAllowed(ctx, "got %v, want %v", "pkA", "pkB")
	err = roundTrip(t, err)
	_, _, pkA, _, _ := security.ParamsErrPublicKeyNotAllowed(err)
	if got, want := pkA, "pkA"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	err = security.ErrorfEndpointAuthorizationFailed(ctx, "ep %v, remote %v, rejected %v", "endpointA", []string{"rem1", "rem2"}, []security.RejectedBlessing{
		{"blessing", fmt.Errorf("not blessed enough")},
	})
	err = roundTrip(t, err)
	_, _, ep, remote, _, _ := security.ParamsErrEndpointAuthorizationFailed(err)
	if got, want := ep, "endpointA"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := remote, []string{"rem1", "rem2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Note that this error has no parameters so it doesn't make sense
	// to use the ParamsErr...
	err = security.ErrorfConstCaveatValidation(ctx, "thouh shall not pass")
	err = roundTrip(t, err)
	if got, want := err.Error(), "security.test: thouh shall not pass"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if !errors.Is(err, security.ErrConstCaveatValidation) {
		t.Errorf("wrong error")
	}
}

func TestCaveatErrors(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	origID, err := uniqueid.Random()
	if err != nil {
		t.Fatal(err)
	}

	err = security.ErrorfCaveatNotRegistered(ctx, "%v", origID)
	err = roundTrip(t, err)
	_, _, id, _ := security.ParamsErrCaveatNotRegistered(err)
	if got, want := origID, id; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	err = security.ErrorfCaveatParamAny(ctx, "%v", origID)
	err = roundTrip(t, err)
	_, _, id, _ = security.ParamsErrCaveatParamAny(err)
	if got, want := origID, id; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	err = security.ErrorfCaveatParamTypeMismatch(ctx, "%v: %v != %v", origID, vdl.TypeOf(origID), vdl.TypeOf(ctx))
	err = roundTrip(t, err)
	_, _, id, typA, typB, _ := security.ParamsErrCaveatParamTypeMismatch(err)
	if got, want := origID, id; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := typA, vdl.TypeOf(origID); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := typB, vdl.TypeOf(ctx); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	err = security.ErrorfCaveatParamCoding(ctx, "%v: %v: %v", origID, vdl.TypeOf(origID), fmt.Errorf("oops"))
	err = roundTrip(t, err)
	_, _, _, _, nerr, _ := security.ParamsErrCaveatParamCoding(err)
	if got, want := nerr.Error(), "oops"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	err = security.ErrorfCaveatValidation(ctx, "%v: ", fmt.Errorf("errr"))
	_, _, nerr, _ = security.ParamsErrCaveatValidation(err)
	if got, want := nerr.Error(), "errr"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	now := time.Now()
	then := now.Add(time.Hour)
	err = security.ErrorfExpiryCaveatValidation(ctx, "now %v, then %v", now, then)
	err = roundTrip(t, err)
	_, _, errNow, errThen, _ := security.ParamsErrExpiryCaveatValidation(err)
	if got, want := errNow, now; !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := errThen, then; !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}

	err = security.ErrorfMethodCaveatValidation(ctx, "%v: allowed %v", "method", []string{"other", "method"})
	err = roundTrip(t, err)
	_, _, invoked, allowed, _ := security.ParamsErrMethodCaveatValidation(err)
	if got, want := invoked, "method"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := len(allowed), 2; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	err = security.ErrorfPeerBlessingsCaveatValidation(ctx, "%v: allowed %v", []string{"pattern..."}, []security.BlessingPattern{"p1...", "p2..."})
	err = roundTrip(t, err)
	_, _, peerPatterns, allowedPatterns, _ := security.ParamsErrPeerBlessingsCaveatValidation(err)
	if got, want := peerPatterns, []string{"pattern..."}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := allowedPatterns, []security.BlessingPattern{"p1...", "p2..."}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestSignatureInterop(t *testing.T) {
	message := make([]byte, 100)
	if _, err := rand.Read(message); err != nil {
		t.Fatal(err)
	}
	signer := sectest.NewECDSASigner(t, elliptic.P384())
	sig, err := signer.Sign([]byte("test"), message)
	if err != nil {
		t.Fatal(err)
	}
	osig := security.EcdsaOnlySignature{
		Purpose: sig.Purpose,
		Hash:    sig.Hash,
		R:       sig.R,
		S:       sig.S,
	}
	buf, err := vom.Encode(osig)
	if err != nil {
		t.Fatal(err)
	}
	nsig := &security.Signature{}
	if err := vom.Decode(buf, &nsig); err != nil {
		t.Fatal(err)
	}

	for i, tc := range []struct {
		got, want []byte
	}{
		{nsig.Purpose, sig.Purpose},
		{nsig.R, sig.R},
		{nsig.S, sig.S},
	} {
		if !bytes.Equal(tc.got, tc.want) {
			t.Errorf("field %v: not equal", i)
		}
	}

	if got, want := nsig.Hash, sig.Hash; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if sig.Ed25519 != nil {
		t.Errorf("should be nil")
	}

	if sig.Rsa != nil {
		t.Errorf("should be nil")
	}

}
