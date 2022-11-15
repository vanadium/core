// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"reflect"
	"testing"
	"time"

	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

func TestCopyParams(t *testing.T) {
	when := time.Now()
	cpy := &security.CallParams{}
	orig := &security.CallParams{
		Timestamp: when,
		Method:    "method",
	}
	call := security.NewCall(orig)
	cpy.Copy(call)
	if got, want := cpy, orig; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	s1 := sectestdata.V23Signer(keys.ECDSA256, sectestdata.V23KeySetA)
	p := sectest.NewPrincipalRootsOnly(t, s1)
	cav := sectest.NewPublicKeyUnconstrainedCaveat(t, p, "peoria")
	cav3p, _ := security.NewPublicKeyCaveat(p.PublicKey(), "somewhere", security.ThirdPartyRequirements{}, sectest.NewExpiryCaveat(t, time.Now().Add(time.Second)))
	discharge, err := p.MintDischarge(cav, security.UnconstrainedUse(), cav3p)
	if err != nil {
		t.Fatal(err)
	}
	orig = &security.CallParams{
		Timestamp:       when,
		Method:          "method",
		MethodTags:      []*vdl.Value{vdl.StringValue(nil, "oops")},
		Suffix:          "/",
		LocalDischarges: security.Discharges{discharge},
	}
	call = security.NewCall(orig)
	cpy = &security.CallParams{}
	cpy.Copy(call)
	if got, want := cpy, orig; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}
