// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

// mockOAuth is a mock OAuthProvider for use in tests.
type mockOAuth struct {
	email    string
	clientID string
}

func NewMockOAuth(mockEmail, mockClientID string) OAuthProvider {
	return &mockOAuth{email: mockEmail, clientID: mockClientID}
}

func (m *mockOAuth) AuthURL(redirectURL string, state string, _ AuthURLApproval) string {
	return redirectURL + "?state=" + state
}

func (m *mockOAuth) ExchangeAuthCodeForEmail(string, string) (string, error) {
	return m.email, nil
}

func (m *mockOAuth) GetEmailAndClientID(string) (string, string, error) {
	return m.email, m.clientID, nil
}
