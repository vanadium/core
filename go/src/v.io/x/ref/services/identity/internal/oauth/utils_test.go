// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

import (
	"strings"
	"testing"
)

func TestClientIDAndSecretFromJSON(t *testing.T) {
	json := `{"web":{"auth_uri":"https://accounts.google.com/o/oauth2/auth","client_secret":"SECRET","token_uri":"https://accounts.google.com/o/oauth2/token","client_email":"EMAIL","redirect_uris":["http://redirecturl"],"client_id":"ID","auth_provider_x509_cert_url":"https://www.googleapis.com/oauth2/v1/certs","javascript_origins":["http://javascriptorigins"]}}`
	id, secret, err := ClientIDAndSecretFromJSON(strings.NewReader(json))
	if err != nil {
		t.Error(err)
	}
	if id != "ID" {
		t.Errorf("Got %q want %q", id, "ID")
	}
	if secret != "SECRET" {
		t.Errorf("Got %q want %q", secret, "SECRET")
	}
}
