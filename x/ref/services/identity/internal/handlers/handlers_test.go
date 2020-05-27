// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/security"
	"v.io/v23/vom"

	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/identity"
	"v.io/x/ref/services/identity/internal/oauth"
	"v.io/x/ref/services/identity/internal/revocation"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func TestBlessingRootJSON(t *testing.T) {
	// TODO(ashankar,ataly): Handle multiple root names?
	blessingNames := []string{"test-root"}
	p := testutil.NewPrincipal(blessingNames...)

	ts := httptest.NewServer(BlessingRoot{p})
	defer ts.Close()
	response, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	dec := json.NewDecoder(response.Body)
	var res identity.BlessingRootResponse
	if err := dec.Decode(&res); err != nil {
		t.Fatal(err)
	}

	// Check that the names are correct.
	sort.Strings(blessingNames)
	sort.Strings(res.Names)
	if !reflect.DeepEqual(res.Names, blessingNames) {
		t.Errorf("Response has incorrect name. Got %v, want %v", res.Names, blessingNames)
	}

	// Check that the public key is correct.
	gotMarshalled, err := base64.URLEncoding.DecodeString(res.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	got, err := security.UnmarshalPublicKey(gotMarshalled)
	if err != nil {
		t.Fatal(err)
	}
	if want := p.PublicKey(); !reflect.DeepEqual(got, want) {
		t.Errorf("Response has incorrect public key.  Got %v, want %v", got, want)
	}
}

func TestBlessingRootBase64VOM(t *testing.T) {
	blessingNames := []string{"alpha"}
	p := testutil.NewPrincipal(blessingNames...)

	ts := httptest.NewServer(BlessingRoot{p})
	defer ts.Close()
	response, err := http.Get(ts.URL + "?output=base64vom")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var root security.Blessings
	if body, err := ioutil.ReadAll(response.Body); err != nil {
		t.Fatal(err)
	} else if v, err := base64.URLEncoding.DecodeString(string(body)); err != nil {
		t.Fatal(err)
	} else if err = vom.Decode(v, &root); err != nil {
		t.Fatal(err)
	}
	if want, _ := p.BlessingStore().Default(); !reflect.DeepEqual(root, want) {
		t.Errorf("Got %v, want %v", root, want)
	}
}

func TestBless(t *testing.T) { //nolint:gocyclo
	var (
		blesserPrin = testutil.NewPrincipal("blesser")
		blesseePrin = testutil.NewPrincipal("blessee")

		methodCav, _ = security.NewMethodCaveat("foo")
		expiryCav, _ = security.NewExpiryCaveat(time.Now().Add(time.Hour))

		mkReqURL = func(baseURLStr string, caveats []security.Caveat, outputFormat string) string {
			baseURL, err := url.Parse(baseURLStr)
			if err != nil {
				t.Fatal(err)
			}
			params := url.Values{}

			if len(caveats) != 0 {
				caveatsVom, err := vom.Encode(caveats)
				if err != nil {
					t.Fatal(err)
				}
				params.Add(caveatsFormKey, base64.URLEncoding.EncodeToString(caveatsVom))
			}
			keyBytes, err := blesseePrin.PublicKey().MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			params.Add(publicKeyFormKey, base64.URLEncoding.EncodeToString(keyBytes))
			params.Add(tokenFormKey, "mocktoken")
			params.Add(outputFormatFormKey, outputFormat)

			baseURL.RawQuery = params.Encode()
			return baseURL.String()
		}

		vomRoundTrip = func(in, out interface{}) error {
			data, err := vom.Encode(in)
			if err != nil {
				return err
			}
			return vom.Decode(data, out)
		}

		decodeBlessings = func(b []byte, outputFormat string) security.Blessings {
			if len(outputFormat) == 0 {
				outputFormat = base64VomFormat
			}
			var res security.Blessings
			switch outputFormat {
			case base64VomFormat:
				if raw, err := base64.URLEncoding.DecodeString(string(b)); err != nil {
					t.Fatal(err)
				} else if err = vom.Decode(raw, &res); err != nil {
					t.Fatal(err)
				}
			case jsonFormat:
				var wb security.WireBlessings
				if err := json.Unmarshal(b, &wb); err != nil {
					t.Fatal(err)
				} else if err = vomRoundTrip(wb, &res); err != nil {
					t.Fatal(err)
				}
			}
			return res
		}
	)

	trustedBlessing, err := blesserPrin.BlessSelf("trustedBlesser")
	if err != nil {
		t.Fatal(err)
	}
	_, err = blesserPrin.BlessingStore().Set(trustedBlessing, "trusted")
	if err != nil {
		t.Fatal(err)
	}
	_, err = blesserPrin.BlessingStore().Set(security.Blessings{}, "...")
	if err != nil {
		t.Fatal(err)
	}

	ctx, shutdown := test.V23Init()
	defer shutdown()
	if ctx, err = v23.WithPrincipal(ctx, blesserPrin); err != nil {
		t.Fatal(err)
	}

	// Make the blessee trust the blesser's roots
	blesserB, _ := blesserPrin.BlessingStore().Default()
	if err := security.AddToRoots(blesseePrin, blesserB); err != nil {
		t.Fatal(err)
	}
	if err := security.AddToRoots(blesseePrin, trustedBlessing); err != nil {
		t.Fatal(err)
	}

	testEmail := "foo@bar.com"
	testClientID := "test-client-id"
	revocationManager := revocation.NewMockRevocationManager(ctx)
	oauthProvider := oauth.NewMockOAuth(testEmail, testClientID)

	testcases := []struct {
		params   OAuthBlesserParams
		caveats  []security.Caveat
		clients  RegisteredAppMap
		blessing string
	}{
		{
			OAuthBlesserParams{
				OAuthProvider:     oauthProvider,
				RevocationManager: revocationManager,
			},
			[]security.Caveat{expiryCav, methodCav},
			RegisteredAppMap{
				"test-client-id": {
					Extension: "{email}:trusted-client",
				},
			},
			"blesser:foo@bar.com:trusted-client",
		},
		{
			OAuthBlesserParams{
				OAuthProvider:    oauthProvider,
				BlessingDuration: 24 * time.Hour,
			},
			nil,
			nil,
			"blesser:test-client-id:foo@bar.com",
		},
		{
			OAuthBlesserParams{
				OAuthProvider:     oauthProvider,
				RevocationManager: revocationManager,
			},
			nil,
			nil,
			"blesser:test-client-id:foo@bar.com",
		},
		{
			OAuthBlesserParams{
				OAuthProvider:     oauthProvider,
				RevocationManager: revocationManager,
			},
			[]security.Caveat{expiryCav, methodCav},
			RegisteredAppMap{},
			"blesser:test-client-id:foo@bar.com",
		},
	}
	for _, testcase := range testcases {
		for _, outputFormat := range []string{jsonFormat, base64VomFormat, ""} {
			ts := httptest.NewServer(NewOAuthBlessingHandler(ctx, testcase.params, testcase.clients))
			defer ts.Close()

			response, err := http.Get(mkReqURL(ts.URL, testcase.caveats, outputFormat))
			if err != nil {
				t.Fatal(err)
			}
			b, err := ioutil.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			blessings := decodeBlessings(b, outputFormat)

			// Blessing should be bound to the blessee.
			if got, want := blessings.PublicKey(), blesseePrin.PublicKey(); !reflect.DeepEqual(got, want) {
				t.Errorf("got blessings for public key %v, want blessings for public key %v", got, want)
			}

			// Verify the name and caveats on the blessings.
			if got, want := security.BlessingNames(blesseePrin, blessings), []string{testcase.blessing}; !reflect.DeepEqual(got, want) {
				t.Errorf("Got %v, want %v", got, want)
			}
			caveats, err := extractCaveats(blessings)
			if err != nil {
				t.Error(err)
				continue
			}
			switch {
			case len(testcase.caveats) > 0:
				// The blessing must have exactly those caveats that were provided in the request.
				if !caveatsMatch(t, caveats, testcase.caveats) {
					t.Errorf("got blessings with caveats %v, want blessings with caveats %v", caveats, testcase.caveats)
				}
			case len(caveats) != 1:
				t.Errorf("got blessings with %d caveats, want blessings with 1 caveats", len(caveats))
			case testcase.params.RevocationManager != nil && caveats[0].Id != security.PublicKeyThirdPartyCaveat.Id:
				// The blessing must have a third-party revocation caveat.
				t.Errorf("got blessings with caveat (%v), want blessings with a PublicKeyThirdPartyCaveat", caveats[0].Id)
			case testcase.params.RevocationManager == nil && caveats[0].Id != security.ExpiryCaveat.Id:
				// The blessing must have an expiry caveat.
				t.Errorf("got blessings with caveat (%v), want blessings with an ExpiryCaveat", caveats[0].Id)
			}
		}
	}
}

func extractCaveats(b security.Blessings) ([]security.Caveat, error) {
	// Extract the wire encoding of the blessings and fish them out.
	bytes, err := vom.Encode(b)
	if err != nil {
		return nil, err
	}
	var wire security.WireBlessings
	if err := vom.Decode(bytes, &wire); err != nil {
		return nil, err
	}
	if got, want := len(wire.CertificateChains), 1; got != want {
		return nil, fmt.Errorf("Got %d blessings, want %d", got, want)
	}
	var ret []security.Caveat
	for _, chain := range wire.CertificateChains {
		for _, cert := range chain {
			ret = append(ret, cert.Caveats...)
		}
	}
	return ret, nil
}

type caveatsSorter struct {
	caveats []security.Caveat
	t       *testing.T
}

func (c caveatsSorter) Len() int      { return len(c.caveats) }
func (c caveatsSorter) Swap(i, j int) { c.caveats[i], c.caveats[j] = c.caveats[j], c.caveats[i] }
func (c caveatsSorter) Less(i, j int) bool {
	bi, err := vom.Encode(c.caveats[i])
	if err != nil {
		c.t.Fatal(err)
	}
	bj, err := vom.Encode(c.caveats[j])
	if err != nil {
		c.t.Fatal(err)
	}
	return bytes.Compare(bi, bj) == -1
}

func caveatsMatch(t *testing.T, got, want []security.Caveat) bool {
	if len(got) != len(want) {
		return false
	}
	g, w := caveatsSorter{got, t}, caveatsSorter{want, t}
	sort.Sort(g)
	sort.Sort(w)
	return reflect.DeepEqual(g, w)
}
