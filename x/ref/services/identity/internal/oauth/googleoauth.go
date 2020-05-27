// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"

	"v.io/v23/context"
)

// googleOAuth implements the OAuthProvider interface with google oauth 2.0.
type googleOAuth struct {
	// client_id and client_secret registered with the Google Developer
	// Console for API access.
	clientID, clientSecret   string
	scope, authURL, tokenURL string
	// URL used to verify google tokens.
	// (From https://developers.google.com/accounts/docs/OAuth2Login#validatinganidtoken
	// and https://developers.google.com/accounts/docs/OAuth2UserAgent#validatetoken)
	verifyURL string

	ctx *context.T
}

func NewGoogleOAuth(ctx *context.T, configFile string) (OAuthProvider, error) {
	clientID, clientSecret, err := getOAuthClientIDAndSecret(configFile)
	if err != nil {
		return nil, err
	}
	return &googleOAuth{
		clientID:     clientID,
		clientSecret: clientSecret,
		scope:        "email",
		authURL:      "https://accounts.google.com/o/oauth2/auth",
		tokenURL:     "https://accounts.google.com/o/oauth2/token",
		verifyURL:    "https://www.googleapis.com/oauth2/v1/tokeninfo?",
		ctx:          ctx,
	}, nil
}

func (g *googleOAuth) AuthURL(redirectURL, state string, approval AuthURLApproval) string {
	var opts []oauth2.AuthCodeOption
	if approval == ExplicitApproval {
		opts = append(opts, oauth2.ApprovalForce)
	}
	return g.oauthConfig(redirectURL).AuthCodeURL(state, opts...)
}

// ExchangeAuthCodeForEmail exchanges the authorization code (which must
// have been obtained with scope=email) for an OAuth token and then uses Google's
// tokeninfo API to extract the email address from that token.
func (g *googleOAuth) ExchangeAuthCodeForEmail(authcode string, url string) (string, error) {
	config := g.oauthConfig(url)
	t, err := config.Exchange(gocontext.TODO(), authcode)
	if err != nil {
		return "", fmt.Errorf("failed to exchange authorization code for token: %v", err)
	}

	if !t.Valid() {
		return "", fmt.Errorf("oauth2 token invalid")
	}
	// Ideally, would validate the token ourselves without an HTTP roundtrip.
	// However, for now, as per:
	// https://developers.google.com/accounts/docs/OAuth2Login#validatinganidtoken
	// pay an HTTP round-trip to have Google do this.
	idToken, ok := t.Extra("id_token").(string)
	if !ok {
		return "", fmt.Errorf("no GoogleIDToken found in OAuth token")
	}
	// The GoogleIDToken is currently validated by sending an HTTP request to
	// googleapis.com.  This adds a round-trip and service may be denied by
	// googleapis.com if this handler becomes a breakout success and receives tons
	// of traffic.  If either is a concern, the GoogleIDToken can be validated
	// without an additional HTTP request.
	// See: https://developers.google.com/accounts/docs/OAuth2Login#validatinganidtoken
	tinfo, err := http.Get(g.verifyURL + "id_token=" + idToken)
	if err != nil {
		return "", fmt.Errorf("failed to talk to GoogleIDToken verifier (%q): %v", g.verifyURL, err)
	}
	defer tinfo.Body.Close()
	if tinfo.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to verify GoogleIDToken: %s", tinfo.Status)
	}
	var gtoken token
	if err := json.NewDecoder(tinfo.Body).Decode(&gtoken); err != nil {
		return "", fmt.Errorf("invalid JSON response from Google's tokeninfo API: %v", err)
	}
	// We check both "verified_email" and "email_verified" here because the token response sometimes
	// contains one and sometimes contains the other.
	if !gtoken.VerifiedEmail && !gtoken.EmailVerified {
		return "", fmt.Errorf("email not verified: %#v", gtoken)
	}
	if gtoken.Issuer != "accounts.google.com" {
		return "", fmt.Errorf("invalid issuer: %v", gtoken.Issuer)
	}
	if gtoken.Audience != config.ClientID {
		return "", fmt.Errorf("unexpected audience(%v) in GoogleIDToken", gtoken.Audience)
	}
	return gtoken.Email, nil
}

// GetEmailAndClientID uses Google's tokeninfo API to determine the email and clientID
// associated with the token.
func (g *googleOAuth) GetEmailAndClientID(accessToken string) (string, string, error) {
	// As per https://developers.google.com/accounts/docs/OAuth2UserAgent#validatetoken
	// we obtain the 'info' for the token via an HTTP roundtrip to Google.
	tokeninfo, err := http.Get(g.verifyURL + "access_token=" + accessToken)
	if err != nil {
		return "", "", fmt.Errorf("unable to use token: %v", err)
	}
	defer tokeninfo.Body.Close()
	if tokeninfo.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unable to verify access token, OAuth2 TokenInfo endpoint responded with StatusCode: %v", tokeninfo.StatusCode)
	}
	// tokeninfo contains a JSON-encoded struct
	var token struct {
		IssuedTo      string `json:"issued_to"`
		Audience      string `json:"audience"`
		UserID        string `json:"user_id"`
		Scope         string `json:"scope"`
		ExpiresIn     int64  `json:"expires_in"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		EmailVerified bool   `json:"email_verified"`
		AccessType    string `json:"access_type"`
	}
	if err := json.NewDecoder(tokeninfo.Body).Decode(&token); err != nil {
		return "", "", fmt.Errorf("invalid JSON response from Google's tokeninfo API: %v", err)
	}
	// We check both "verified_email" and "email_verified" here because the token response sometimes
	// contains one and sometimes contains the other.
	if !token.VerifiedEmail && !token.EmailVerified {
		return "", "", fmt.Errorf("email not verified")
	}
	return token.Email, token.Audience, nil
}

func (g *googleOAuth) oauthConfig(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     g.clientID,
		ClientSecret: g.clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{g.scope},
		Endpoint: oauth2.Endpoint{
			AuthURL:  g.authURL,
			TokenURL: g.tokenURL,
		},
	}
}

func getOAuthClientIDAndSecret(configFile string) (clientID, clientSecret string, err error) {
	f, err := os.Open(configFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to open %q: %v", configFile, err)
	}
	defer f.Close()
	clientID, clientSecret, err = ClientIDAndSecretFromJSON(f)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode JSON in %q: %v", configFile, err)
	}
	return clientID, clientSecret, nil
}

// IDToken JSON message returned by Google's verification endpoint.
//
// This differs from the description in:
// https://developers.google.com/accounts/docs/OAuth2Login#obtainuserinfo
// because the Google tokeninfo endpoint
// (https://www.googleapis.com/oauth2/v1/tokeninfo?id_token=XYZ123)
// mentioned in:
// https://developers.google.com/accounts/docs/OAuth2Login#validatinganidtoken
// seems to return the following JSON message.
type token struct {
	Issuer        string `json:"issuer"`
	IssuedTo      string `json:"issued_to"`
	Audience      string `json:"audience"`
	UserID        string `json:"user_id"`
	ExpiresIn     int64  `json:"expires_in"`
	IssuedAt      int64  `json:"issued_at"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	EmailVerified bool   `json:"email_verified"`
}
