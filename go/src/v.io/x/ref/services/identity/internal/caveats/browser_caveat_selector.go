// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package caveats

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"v.io/x/ref/services/identity/internal/templates"

	"v.io/v23/security"
)

var ErrSeekblessingsCancelled = fmt.Errorf("seekblessings has been cancelled")

type browserCaveatSelector struct {
	assetsPrefix string
}

// NewBrowserCaveatSelector returns a caveat selector that renders a form in the
// to accept user caveat selections.
func NewBrowserCaveatSelector(assetsPrefix string) CaveatSelector {
	return &browserCaveatSelector{assetsPrefix}
}

func (s *browserCaveatSelector) Render(blessingName, state, redirectURL string, w http.ResponseWriter, r *http.Request) error {
	tmplargs := struct {
		BlessingName, Macaroon, MacaroonURL, AssetsPrefix string
	}{blessingName, state, redirectURL, s.assetsPrefix}
	w.Header().Set("Context-Type", "text/html")
	if err := templates.SelectCaveats.Execute(w, tmplargs); err != nil {
		return err
	}
	return nil
}

func (s *browserCaveatSelector) ParseSelections(r *http.Request) (caveats []CaveatInfo, state string, additionalExtension string, err error) {
	state = r.FormValue("macaroon")
	if r.FormValue("cancelled") == "true" {
		err = ErrSeekblessingsCancelled
		return
	}
	additionalExtension = r.FormValue("blessingExtension")
	if caveats, err = s.caveats(r); err != nil {
		return
	}
	return
}

func (s *browserCaveatSelector) caveats(r *http.Request) ([]CaveatInfo, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	var caveats []CaveatInfo
	for i, cavName := range r.Form["caveat"] {
		var err error
		var caveat CaveatInfo
		switch cavName {
		case "ExpiryCaveat":
			caveat, err = newExpiryCaveatInfo(r.Form[cavName][i], r.FormValue("timezoneOffset"))
		case "MethodCaveat":
			caveat, err = newMethodCaveatInfo(r.Form[cavName][i])
		case "PeerBlessingsCaveat":
			caveat, err = newPeerBlessingsCaveatInfo(r.Form[cavName][i])
		case "RevocationCaveat":
			caveat = newRevocationCaveatInfo()
		default:
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("unable to create caveat %s: %v", cavName, err)
		}
		caveats = append(caveats, caveat)
	}
	if len(caveats) == 0 {
		return nil, fmt.Errorf("server does not allow unconstrained blessings")
	}
	return caveats, nil
}

func newExpiryCaveatInfo(timestamp, utcOffset string) (CaveatInfo, error) {
	var empty CaveatInfo
	t, err := time.Parse("2006-01-02T15:04", timestamp)
	if err != nil {
		return empty, fmt.Errorf("parseTime failed: %v", err)
	}
	// utcOffset is returned as minutes from JS, so we need to parse it to a duration.
	offset, err := time.ParseDuration(utcOffset + "m")
	if err != nil {
		return empty, fmt.Errorf("failed to parse duration: %v", err)
	}
	return CaveatInfo{"Expiry", []interface{}{t.Add(offset)}}, nil
}

func newMethodCaveatInfo(methodsCSV string) (CaveatInfo, error) {
	methods := strings.Split(methodsCSV, ",")
	if len(methods) < 1 {
		return CaveatInfo{}, fmt.Errorf("must pass at least one method")
	}
	var ifaces []interface{}
	for _, m := range methods {
		ifaces = append(ifaces, m)
	}
	return CaveatInfo{"Method", ifaces}, nil
}

func newPeerBlessingsCaveatInfo(patternsCSV string) (CaveatInfo, error) {
	patterns := strings.Split(patternsCSV, ",")
	if len(patterns) < 1 {
		return CaveatInfo{}, fmt.Errorf("must pass at least one peer blessing pattern")
	}
	var ifaces []interface{}
	for _, p := range patterns {
		ifaces = append(ifaces, security.BlessingPattern(p))
	}
	return CaveatInfo{"PeerBlessings", ifaces}, nil
}

func newRevocationCaveatInfo() CaveatInfo {
	return CaveatInfo{Type: "Revocation"}
}
