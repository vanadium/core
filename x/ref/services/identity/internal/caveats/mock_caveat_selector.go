// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package caveats

import (
	"net/http"
	"time"

	"v.io/v23/security"
)

type mockCaveatSelector struct {
	state string
}

// NewMockCaveatSelector returns a CaveatSelector that always returns a default set
// of caveats: [exprity caveat with a 1h expiry, revocation caveat, and a method caveat
// for methods "methodA" and "methodB"] and the additional extension: "test-extension"
// This selector is only meant to be used during testing.
func NewMockCaveatSelector() CaveatSelector {
	return &mockCaveatSelector{}
}

func (s *mockCaveatSelector) Render(_, state, redirectURL string, w http.ResponseWriter, r *http.Request) error {
	s.state = state
	http.Redirect(w, r, redirectURL, http.StatusFound)
	return nil
}

func (s *mockCaveatSelector) ParseSelections(r *http.Request) (caveats []CaveatInfo, state string, additionalExtension string, err error) {
	caveats = []CaveatInfo{
		{"Revocation", []interface{}{}},
		{"Expiry", []interface{}{time.Now().Add(time.Hour)}},
		{"Method", []interface{}{"methodA", "methodB"}},
		{"PeerBlessings", []interface{}{security.BlessingPattern("peerA"), security.BlessingPattern("peerB")}},
	}
	state = s.state
	additionalExtension = "test-extension"
	return
}
