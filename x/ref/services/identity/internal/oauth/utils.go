// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

import (
	"encoding/json"
	"fmt"
	"io"
)

// AccessTokenClient represents a client of an OAuthProvider.
type AccessTokenClient struct {
	// Descriptive name of the client.
	Name string
	// OAuth Client ID.
	ClientID string
}

// ClientIDFromJSON parses JSON-encoded API access information in 'r' and returns
// the extracted ClientID.
// This JSON-encoded data is typically available as a download from the Google
// API Access console for your application
// (https://code.google.com/apis/console).
func ClientIDFromJSON(r io.Reader) (id string, err error) {
	var data map[string]interface{}
	var typ string
	if data, typ, err = decodeAccessMapFromJSON(r); err != nil {
		return
	}
	var ok bool
	if id, ok = data["client_id"].(string); !ok {
		err = fmt.Errorf("%s.client_id not found", typ)
		return
	}
	return
}

// ClientIDAndSecretFromJSON parses JSON-encoded API access information in 'r'
// and returns the extracted ClientID and ClientSecret.
// This JSON-encoded data is typically available as a download from the Google
// API Access console for your application
// (https://code.google.com/apis/console).
func ClientIDAndSecretFromJSON(r io.Reader) (id, secret string, err error) {
	var data map[string]interface{}
	var typ string
	if data, typ, err = decodeAccessMapFromJSON(r); err != nil {
		return
	}
	var ok bool
	if id, ok = data["client_id"].(string); !ok {
		err = fmt.Errorf("%s.client_id not found", typ)
		return
	}
	if secret, ok = data["client_secret"].(string); !ok {
		err = fmt.Errorf("%s.client_secret not found", typ)
		return
	}
	return
}

// ClientName checks if the provided clientID is present in one of the provided
// 'clients' and if so returns the corresponding client name. It returns an error
// otherwise.
func ClientName(clientID string, clients []AccessTokenClient) (string, error) {
	for _, c := range clients {
		if clientID == c.ClientID {
			return c.Name, nil
		}
	}
	return "", fmt.Errorf("unrecognized client ID, confused deputy? https://developers.google.com/accounts/docs/OAuth2UserAgent#validatetoken")
}

func decodeAccessMapFromJSON(r io.Reader) (data map[string]interface{}, typ string, err error) {
	var full map[string]interface{}
	if err = json.NewDecoder(r).Decode(&full); err != nil {
		return
	}
	var ok bool
	typ = "web"
	if data, ok = full[typ].(map[string]interface{}); !ok {
		typ = "installed"
		if data, ok = full[typ].(map[string]interface{}); !ok {
			err = fmt.Errorf("web or installed configuration not found")
		}
	}
	return
}
