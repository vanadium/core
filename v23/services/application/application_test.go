// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package application

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"v.io/v23/security"
)

func TestEnvelopeJSON(t *testing.T) {
	before := Envelope{
		Title:     "title",
		Args:      []string{"arg1", "arg2"},
		Binary:    SignedFile{File: "binary"},
		Publisher: newBlessing(t, "publisher"),
		Env:       []string{"NAME=value"},
		Packages: map[string]SignedFile{
			"pkg1": SignedFile{File: "pkg1.data"},
			"pkg2": SignedFile{File: "pkg2.config"},
		},
		Restarts:          2,
		RestartTimeWindow: time.Second,
	}
	// If the fields of Envelope have been changed, then the testdata above
	// needs to be updated.  This actually applies recursively to the types
	// of the fields of Envelope. If you have ideas on how to test that,
	// chime in.
	if n := reflect.TypeOf(before).NumField(); n != 8 {
		t.Errorf("It appears that fields have been added to or removed from Envelope but TestEnvelopeJSON has not been updated. Please update it.")
	}

	jsonBytes, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	var after Envelope
	if err := json.Unmarshal(jsonBytes, &after); err != nil {
		t.Fatalf("Unmarshal(%q): %v", jsonBytes, err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("Got %#v, want %#v after JSON roundtripping", after, before)
	}
}

func newBlessing(t *testing.T, name string) security.Blessings {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p, err := security.CreatePrincipal(security.NewInMemoryECDSASigner(key), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.BlessSelf(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
