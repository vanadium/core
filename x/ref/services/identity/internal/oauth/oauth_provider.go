// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

// Option to OAuthProvider.AuthURL controlling whether previously provided user consent can be re-used.
type AuthURLApproval bool

const (
	ExplicitApproval AuthURLApproval = false // Require explicit user consent.
	ReuseApproval    AuthURLApproval = true  // Reuse a previous user consent if possible.
)

// OAuthProvider authenticates users to the identity server via the OAuth2 Web Server flow.
//nolint:golint // API change required.
type OAuthProvider interface {
	// AuthURL is the URL the user must visit in order to authenticate with the OAuthProvider.
	// After authentication, the user will be re-directed to redirectURL with the provided state.
	AuthURL(redirectURL string, state string, approval AuthURLApproval) (url string)
	// ExchangeAuthCodeForEmail exchanges the provided authCode for the email of the
	// authenticated user on behalf of the token has been issued.
	ExchangeAuthCodeForEmail(authCode string, url string) (email string, err error)
	// GetEmailAndClientID returns the email and clientID associated with the token.
	GetEmailAndClientID(accessToken string) (email string, clientID string, err error)
}
