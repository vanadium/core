// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jws"

	"v.io/v23/context"
)

const (
	cookieName = "VanadiumAllocatorCookie"
	// Payload for cookie during the oauth flow.  Should never match a valid
	// email address.
	csrfCookieValue = "csrf"
	csrfTokenLen    = 16
	// This parameter name is hardcorded in static/dash.js,
	// and should be changed in tandem.
	paramCSRF = "csrf"
)

var errOauthInProgress = errors.New("oauth login in process")

type claimSet struct {
	jws.ClaimSet
	Email string `json:"email"`
}

// decodeToken is modeled after golang.org/x/oauth2/jws.Decode.  The only
// difference lies in using claimSet instead of jws.ClaimSet (as of May 2016,
// the latter does not contain the Email field which we need).
func decodeToken(payload string) (*claimSet, error) {
	s := strings.Split(payload, ".")
	if len(s) < 2 {
		return nil, errors.New("jws: invalid token received")
	}
	decoded, err := base64Decode(s[1])
	if err != nil {
		return nil, err
	}
	c := &claimSet{}
	err = json.NewDecoder(bytes.NewBuffer(decoded)).Decode(c)
	return c, err
}

// base64Decode is copied from golang.org/x/oauth2/jws.base64Decode.
func base64Decode(s string) ([]byte, error) {
	// Add back missing padding.
	switch len(s) % 4 {
	case 1:
		s += "==="
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

type oauthCredentials struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	// HashKey is not strictly part of the OAuth credentials, but for
	// convenience we put it in the same object.
	//
	// TODO(caprita): Consider signing cookies with the server's private key
	// (and verify signatures using blessings) instead of maintaining a
	// separate signing key.
	HashKey string `json:"hashKey"`
}

func (c *oauthCredentials) validate() error {
	switch {
	case c.ClientID == "":
		return errors.New("clientId empty")
	case c.ClientSecret == "":
		return errors.New("clientSecret empty")
	case c.HashKey == "":
		return errors.New("hashKey empty")
	default:
		return nil
	}
}

func clientCredsFromFile(f string) (*oauthCredentials, error) {
	jsonData, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}
	creds := new(oauthCredentials)
	if err := json.Unmarshal(jsonData, creds); err != nil {
		return nil, err
	}
	return creds, nil
}

func oauthConfig(externalURL string, oauthCreds *oauthCredentials) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     oauthCreds.ClientID,
		ClientSecret: oauthCreds.ClientSecret,
		RedirectURL:  strings.TrimRight(externalURL, "/") + routeOauth,
		Scopes:       []string{"email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
	}
}

type oauthState struct {
	CSRFToken, RedirectURL string
}

func (s oauthState) encode() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("failed to encode %v: %v", s, err)
	}
	return string(b), nil
}

func (s *oauthState) decode(enc string) error {
	if err := json.Unmarshal([]byte(enc), s); err != nil {
		return fmt.Errorf("failed to decode %v: %v", enc, err)
	}
	return nil
}

func generateCSRFToken(ctx *context.T) string {
	b := make([]byte, csrfTokenLen)
	if _, err := rand.Read(b); err != nil {
		ctx.Errorf("Failed to generate csrf cookie: %v", err)
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

func validateCSRF(ctx *context.T, req *http.Request, baker cookieBaker, csrfToken string) bool {
	cookieToken, cookieCSRFToken, err := baker.get(req, cookieName)
	if cookieToken == "" && err == nil {
		err = errors.New("missing cookie")
	}
	if err != nil {
		ctx.Errorf("Failed to read csrf cookie: %v", err)
		return false
	}
	return cookieToken == csrfCookieValue && cookieCSRFToken == csrfToken
}

func checkSession(baker cookieBaker, r *http.Request, mutating bool) (string, string, error) {
	email, csrfToken, err := baker.get(r, cookieName)
	switch {
	case err == nil && email != "" && email != csrfCookieValue:
		// The user is already logged in.
		if mutating && r.FormValue(paramCSRF) != csrfToken {
			return "", "", errors.New("bad CSRF token for mutating request")
		} else {
			return email, csrfToken, nil
		}
	case err == nil && email == csrfCookieValue:
		// The user is in the middle of the oauth flow.
		//
		// TODO(caprita): Rather then presenting an error page to the
		// user, we can gracefully ask the client to retry a bit later
		// (via a redirect or by sending a 503 with retry-after).  This
		// will invariably trigger a new oauth sequence (since the
		// request's CSRF token will no longer match on the retry).
		return "", "", errOauthInProgress
	case err == nil:
		return "", "", errors.New("missing cookie")
	default:
		return "", "", fmt.Errorf("bad cookie: %v", err)
	}
}

func requireSession(ctx *context.T, oauthCfg *oauth2.Config, baker cookieBaker, w http.ResponseWriter, r *http.Request, mutating bool) (string, string, error) {
	if email, csrfToken, err := checkSession(baker, r, mutating); err == nil || err == errOauthInProgress {
		return email, csrfToken, err
	} else {
		ctx.Infof("Re-authenticating: %v", err)
	}
	// Proceed with the oauth flow.
	csrfToken := generateCSRFToken(ctx)
	if csrfToken == "" {
		return "", "", errors.New("failed to generate CSRF token")
	}
	redirectTo := makeURL(ctx, routeHome, params{paramMessage: "Re-authentication was required."})
	if !mutating {
		redirectTo = r.URL.String()
	}
	s, err := oauthState{CSRFToken: csrfToken, RedirectURL: redirectTo}.encode()
	if err != nil {
		return "", "", fmt.Errorf("failed to encode state: %v", err)
	}
	if err := baker.set(w, cookieName, csrfCookieValue, csrfToken); err != nil {
		return "", "", fmt.Errorf("failed to set CSRF cookie: %v", err)
	}
	authURL := oauthCfg.AuthCodeURL(s)
	http.Redirect(w, r, authURL, http.StatusFound)
	return "", "", nil
}

func handleOauth(ctx *context.T, args httpArgs, baker cookieBaker, w http.ResponseWriter, r *http.Request) {
	const (
		paramState = "state"
		paramCode  = "code"
	)
	var state oauthState
	if err := state.decode(r.FormValue(paramState)); err != nil {
		args.assets.badRequest(ctx, w, r, fmt.Errorf("invalid state: %v", err))
		return
	}
	if token := state.CSRFToken; !validateCSRF(ctx, r, baker, token) {
		args.assets.badRequest(ctx, w, r, fmt.Errorf("invalid csrf token: %v", token))
		return
	}
	code := r.FormValue(paramCode)
	oauthCfg := oauthConfig(args.externalURL, args.oauthCreds)
	t, err := oauthCfg.Exchange(oauth2.NoContext, code)
	if err != nil {
		args.assets.badRequest(ctx, w, r, fmt.Errorf("exchange failed: %v", err))
		return
	}
	idToken, ok := t.Extra("id_token").(string)
	if !ok {
		args.assets.badRequest(ctx, w, r, errors.New("invalid id token"))
		return
	}

	claimSet, err := decodeToken(idToken)
	if err != nil {
		ctx.Errorf("oauth2: error decoding JWT token: %v", err)
		args.assets.errorOccurred(ctx, w, r, routeHome, err)
		return
	}
	email := claimSet.Email
	csrfToken := generateCSRFToken(ctx)
	if err := baker.set(w, cookieName, email, csrfToken); err != nil {
		ctx.Errorf("Failed to set email cookie: %v", err)
		args.assets.errorOccurred(ctx, w, r, routeHome, err)
		return
	}
	if state.RedirectURL == "" {
		args.assets.badRequest(ctx, w, r, errors.New("no redirect url provided"))
		return
	}
	http.Redirect(w, r, state.RedirectURL, http.StatusFound)
}
