// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package auditor

import (
	"reflect"
	"testing"
	"time"

	"v.io/v23/security"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/audit"
	"v.io/x/ref/test"
)

func TestBlessingAuditor(t *testing.T) {
	ctx, cancel := test.TestContext()
	defer cancel()
	auditor, reader := NewMockBlessingAuditor()

	p, err := vsecurity.NewPrincipal()
	if err != nil {
		t.Fatalf("failed to create principal: %v", err)
	}
	expiryCaveat := newCaveat(security.NewExpiryCaveat(time.Now().Add(time.Hour)))
	revocationCaveat := newThirdPartyCaveat(t, p)

	tests := []struct {
		Extension          string
		Email              string
		Caveats            []security.Caveat
		RevocationCaveatID string
		Blessings          security.Blessings
	}{
		{
			Extension:          "foo@bar.com:nocaveats:bar@baz.com",
			Email:              "foo@bar.com",
			RevocationCaveatID: "",
			Blessings:          newBlessing(t, p, "test:foo@bar.com:nocaveats:bar@baz.com"),
		},
		{
			Extension:          "users:foo@bar.com:caveat",
			Email:              "foo@bar.com",
			Caveats:            []security.Caveat{expiryCaveat},
			RevocationCaveatID: "",
			Blessings:          newBlessing(t, p, "test:foo@bar.com:caveat"),
		},
		{
			Extension:          "special:guests:foo@bar.com:caveatAndRevocation",
			Email:              "foo@bar.com",
			Caveats:            []security.Caveat{expiryCaveat, revocationCaveat},
			RevocationCaveatID: revocationCaveat.ThirdPartyDetails().ID(),
			Blessings:          newBlessing(t, p, "test:foo@bar.com:caveatAndRevocation"),
		},
	}

	for _, test := range tests {
		args := []interface{}{nil, nil, test.Extension}
		for _, cav := range test.Caveats {
			args = append(args, cav)
		}
		if err := auditor.Audit(ctx, audit.Entry{
			Method:    "Bless",
			Arguments: args,
			Results:   []interface{}{test.Blessings},
		}); err != nil {
			t.Errorf("Failed to audit Blessing %v: %v", test.Blessings, err)
		}
		ch := reader.Read(ctx, "query")
		got := <-ch
		if got.Email != test.Email {
			t.Errorf("got %v, want %v", got.Email, test.Email)
		}
		if !reflect.DeepEqual(got.Caveats, test.Caveats) {
			t.Errorf("got %#v, want %#v", got.Caveats, test.Caveats)
		}
		if got.RevocationCaveatID != test.RevocationCaveatID {
			t.Errorf("got %v, want %v", got.RevocationCaveatID, test.RevocationCaveatID)
		}
		if !reflect.DeepEqual(got.Blessings, test.Blessings) {
			t.Errorf("got %v, want %v", got.Blessings, test.Blessings)
		}
		var extra bool
		for range ch {
			// Drain the channel to prevent the producer goroutines from being leaked.
			extra = true
		}
		if extra {
			t.Errorf("Got more entries that expected for test %+v", test)
		}
	}
}

func newThirdPartyCaveat(t *testing.T, p security.Principal) security.Caveat {
	tp, err := security.NewPublicKeyCaveat(p.PublicKey(), "location", security.ThirdPartyRequirements{}, newCaveat(security.NewMethodCaveat("method")))
	if err != nil {
		t.Fatal(err)
	}
	return tp
}

func newBlessing(t *testing.T, p security.Principal, name string) security.Blessings {
	b, err := p.BlessSelf(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newCaveat(caveat security.Caveat, err error) security.Caveat {
	if err != nil {
		panic(err)
	}
	return caveat
}
