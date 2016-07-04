// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file provides utilities for exchanging an OAuth token for a blessing via
// the Vanadium identity provider, and initializing a Vanadium context with this
// blessing.
//
// See v.io/x/ref/services/syncbase/bridge/mojo/blessings.go for more
// documentation.

package bridge

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	seclib "v.io/x/ref/lib/security"
)

// SeekAndSetBlessings exchanges an OAuth token for blessings from the dev.v.io
// server. Currently, oauthProvider must be set to "google".
func SeekAndSetBlessings(ctx *context.T, oauthProvider string, oauthToken string) error {
	if strings.ToLower(oauthProvider) != "google" {
		return fmt.Errorf("unsupported oauthProvider %q; currently, \"google\" is the only supported provider", oauthProvider)
	}
	p := v23.GetPrincipal(ctx)
	blessings, err := token2blessings(oauthToken, p.PublicKey())
	if err != nil {
		return err
	}
	ctx.Infof("Obtained blessings %v", blessings)
	if err := seclib.SetDefaultBlessings(p, blessings); err != nil {
		return fmt.Errorf("failed to use blessings: %v", err)
	}
	return nil
}

func token2blessings(token string, key security.PublicKey) (security.Blessings, error) {
	var ret security.Blessings
	url, err := token2blessingURL(token, key)
	if err != nil {
		return ret, err
	}

	resp, err := http.Get(url)
	if err != nil {
		return ret, fmt.Errorf("HTTP request to exchange OAuth token for blessings failed: %v", err)
	}

	defer resp.Body.Close()
	b64bytes, err := ioutil.ReadAll(resp.Body)

	vombytes, err := base64.URLEncoding.DecodeString(string(b64bytes))
	if err != nil {
		return ret, fmt.Errorf("invalid base64 encoded blessings: %v", err)
	}
	if err := vom.Decode(vombytes, &ret); err != nil {
		return ret, fmt.Errorf("invalid encoded blessings: %v", err)
	}
	return ret, nil
}

func token2blessingURL(token string, key security.PublicKey) (string, error) {
	base, err := url.Parse("https://dev.v.io/auth/google/bless")
	if err != nil {
		return "", fmt.Errorf("failed to parse blessing URL: %v", err)
	}
	pub, err := key.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("invalid public key: %v", err)
	}

	params := url.Values{}
	params.Add("public_key", base64.URLEncoding.EncodeToString(pub))
	params.Add("token", token)
	params.Add("output_format", "base64vom")

	base.RawQuery = params.Encode()
	return base.String(), nil
}
