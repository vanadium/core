// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conventions

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/security"
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
		if got := ParseUserId(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseUserId(%s) wanted %v, got %v", tt.in, tt.want, got)
		}
	}
}

func TestGetClientUserIds(t *testing.T) {
	ctx, shutdown := context.RootContext()
	defer shutdown()

	// Bob is the remote principal (and the local one too)
	bob := newPrincipal(t)
	blessings, err := bob.BlessSelf("idt:u:bob:memyselfandi")
	if err != nil {
		t.Errorf("blessing myself %s", err)
	}

	// Server trust client.
	call := security.NewCall(&security.CallParams{RemoteBlessings: blessings, LocalPrincipal: bob})
	if want, got := []string{"idt:u:bob"}, GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}

	// Server doesn't trust client.
	call = security.NewCall(&security.CallParams{RemoteBlessings: blessings})
	if want, got := []string{UnauthenticatedUser}, GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}

	// Server trusts client but blessing is of the wrong format.
	blessings, err = bob.BlessSelf("idt:u")
	if err != nil {
		t.Errorf("blessing myself %s", err)
	}
	call = security.NewCall(&security.CallParams{RemoteBlessings: blessings, LocalPrincipal: bob})
	if want, got := []string{"idt"}, GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}

	// Server trusts client and has same public key.
	blessings, err = bob.BlessSelf("idt:u")
	if err != nil {
		t.Errorf("blessing myself %s", err)
	}
	call = security.NewCall(&security.CallParams{RemoteBlessings: blessings, LocalBlessings: blessings, LocalPrincipal: bob})
	if want, got := []string{"idt", ServerUser}, GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}

	// Server trusts client and has same public key and client supplies a user id.
	blessings, err = bob.BlessSelf("idt:u:bob")
	if err != nil {
		t.Errorf("blessing myself %s", err)
	}
	call = security.NewCall(&security.CallParams{RemoteBlessings: blessings, LocalBlessings: blessings, LocalPrincipal: bob})
	if want, got := []string{"idt:u:bob", ServerUser}, GetClientUserIds(ctx, call); !reflect.DeepEqual(got, want) {
		t.Errorf("GetClientUserIds() wanted %v, got %v", want, got)
	}
}

func newPrincipal(t *testing.T) security.Principal {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p, err := security.CreatePrincipal(
		security.NewInMemoryECDSASigner(key),
		nil,
		&trustAllRoots{dump: make(map[security.BlessingPattern][]security.PublicKey)},
	)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type trustAllRoots struct {
	dump map[security.BlessingPattern][]security.PublicKey
}

func (r *trustAllRoots) Add(root []byte, pattern security.BlessingPattern) error {
	key, err := security.UnmarshalPublicKey(root)
	if err != nil {
		return err
	}
	r.dump[pattern] = append(r.dump[pattern], key)
	return nil
}
func (r *trustAllRoots) Recognized(root []byte, blessing string) error {
	return nil
}
func (r *trustAllRoots) Dump() map[security.BlessingPattern][]security.PublicKey {
	return r.dump
}
func (r *trustAllRoots) DebugString() string {
	return fmt.Sprintf("%v", r)
}
