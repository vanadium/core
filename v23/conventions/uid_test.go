// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conventions_test

import (
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/x/ref/test/sectestdata"
)

func TestParseUserId(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"dev.v.io", []string{"dev.v.io"}},
		{"dev.v.io:u", []string{"dev.v.io"}},
		{"dev.v.io:u:p@google.com", []string{"dev.v.io", "u", "p@google.com"}},
		{"dev.v.io:uu:p@google.com", []string{"dev.v.io"}},
		{"dev.v.io:u:p@google.com:x", []string{"dev.v.io", "u", "p@google.com"}},
		{"dev.v.io:uu:p@google.com:x", []string{"dev.v.io"}},
	}
	for _, tt := range tests {
		if got := conventions.ParseUserId(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseUserId(%s) wanted %v, got %v", tt.in, tt.want, got)
		}
	}
}

func TestGetClientUserIds(t *testing.T) {
	for _, kt := range sectestdata.SupportedKeyAlgos {
		signer := sectestdata.V23Signer(kt, sectestdata.V23KeySetA)
		p := sectest.NewPrincipal(t, signer, nil, &sectest.TrustAllRoots{})
		testGetClientUserIds(t, p)
	}
}

func testGetClientUserIds(t *testing.T, bob security.Principal) {
	ctx, shutdown := context.RootContext()
	defer shutdown()

	// Bob is the remote principal (and the local one too)
	blessings, err := bob.BlessSelf("idt:u:bob:memyselfandi")
	if err != nil {
		t.Errorf("blessing myself %s", err)
	}

	// Server trust client.
	call := security.NewCall(&security.CallParams{RemoteBlessings: blessings, LocalPrincipal: bob})
	if want, got := []string{"idt:u:bob"}, conventions.GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}

	// Server doesn't trust client.
	call = security.NewCall(&security.CallParams{RemoteBlessings: blessings})
	if want, got := []string{conventions.UnauthenticatedUser}, conventions.GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}

	// Server trusts client but blessing is of the wrong format.
	blessings, err = bob.BlessSelf("idt:u")
	if err != nil {
		t.Errorf("blessing myself %s", err)
	}
	call = security.NewCall(&security.CallParams{RemoteBlessings: blessings, LocalPrincipal: bob})
	if want, got := []string{"idt"}, conventions.GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}

	// Server trusts client and has same public key.
	blessings, err = bob.BlessSelf("idt:u")
	if err != nil {
		t.Errorf("blessing myself %s", err)
	}
	call = security.NewCall(&security.CallParams{RemoteBlessings: blessings, LocalBlessings: blessings, LocalPrincipal: bob})
	if want, got := []string{"idt", conventions.ServerUser}, conventions.GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}

	// Server trusts client and has same public key and client supplies a user id.
	blessings, err = bob.BlessSelf("idt:u:bob")
	if err != nil {
		t.Errorf("blessing myself %s", err)
	}
	call = security.NewCall(&security.CallParams{RemoteBlessings: blessings, LocalBlessings: blessings, LocalPrincipal: bob})
	if want, got := []string{"idt:u:bob", conventions.ServerUser}, conventions.GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}
}
