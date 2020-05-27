// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/ref/services/identity/internal/util"
)

// BlessingRoot is an http.Handler implementation that renders the server's
// blessing names and public key in a json string.
type BlessingRoot struct {
	P security.Principal
}

// Cached response so we don't have to bless and encode every time somebody
// hits this route.
var (
	cacheMu                 sync.RWMutex
	cachedResponseJSON      []byte
	cachedResponseBase64VOM []byte
)

func (b BlessingRoot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch format := r.FormValue("output"); format {
	case "base64vom":
		b.base64vomResponse(w, r)
	default:
		b.jsonResponse(w, r)
	}
}

func (b BlessingRoot) base64vomResponse(w http.ResponseWriter, r *http.Request) {
	cacheMu.RLock()
	if cachedResponseBase64VOM != nil {
		respondString(w, "text/plain", cachedResponseBase64VOM)
		cacheMu.RUnlock()
		return
	}
	cacheMu.RUnlock()
	blessings, _ := b.P.BlessingStore().Default()
	roots := security.RootBlessings(blessings)
	strs := make([]string, len(roots))
	for i, r := range roots {
		v, err := vom.Encode(r)
		if err != nil {
			util.HTTPServerError(w, err)
		}
		strs[i] = base64.URLEncoding.EncodeToString(v)
	}
	ret := []byte(strings.Join(strs, "\n"))
	cacheMu.Lock()
	cachedResponseBase64VOM = ret
	cacheMu.Unlock()
	respondString(w, "text/plain", cachedResponseBase64VOM)
}

func (b BlessingRoot) jsonResponse(w http.ResponseWriter, r *http.Request) {
	cacheMu.RLock()
	if cachedResponseJSON != nil {
		respondString(w, "application/json", cachedResponseJSON)
		cacheMu.RUnlock()
		return
	}
	cacheMu.RUnlock()

	// The identity service itself is blessed by a more protected key.
	// Use the root certificate as the identity provider.
	//
	// TODO(ashankar): This is making the assumption that the identity
	// service has a single blessing, which may not be true in general.
	// Revisit this.
	def, _ := b.P.BlessingStore().Default()
	name, der, err := util.RootCertificateDetails(def)
	if err != nil {
		util.HTTPServerError(w, err)
		return
	}
	str := base64.URLEncoding.EncodeToString(der)

	// TODO(suharshs): Ideally this struct would be BlessingRootResponse but vdl does
	// not currently allow field annotations. Once those are allowed, then use that
	// here.
	rootInfo := struct {
		Names     []string `json:"names"`
		PublicKey string   `json:"publicKey"`
	}{
		Names:     []string{name},
		PublicKey: str,
	}

	res, err := json.Marshal(rootInfo)
	if err != nil {
		util.HTTPServerError(w, err)
		return
	}

	cacheMu.Lock()
	cachedResponseJSON = res
	cacheMu.Unlock()
	respondString(w, "application/json", res)
}

func respondString(w http.ResponseWriter, contentType string, res []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Write(res) //nolint:errcheck
}
