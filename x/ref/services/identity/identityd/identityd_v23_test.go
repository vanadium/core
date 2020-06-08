// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"crypto/tls"
	"encoding/base64"
	_ "encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/services/stats"
	"v.io/v23/vom"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23test"
)

const urlRE = "^(https://.*)$"

func httpGet(t *testing.T, url string) string {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		t.Fatalf("failed to create a cookie jar: %v", err)
	}
	client := &http.Client{
		Jar:       jar,
		Transport: transport,
	}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Get(%q) failed: %v", url, err)
	}
	output, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("ReadAll() failed: %v", err)
	}
	return string(output)
}

func seekBlessings(t *testing.T, sh *v23test.Shell, principal, httpAddr string) {
	args := []string{
		"seekblessings",
		"--browser=false",
		fmt.Sprintf("--from=%s/auth/google", httpAddr),
		"-v=3",
	}
	cmd := sh.Cmd(principal, args...)
	cmd.Start()
	line := cmd.S.ExpectSetEventuallyRE(urlRE)[0][1]
	// Scan the output of "principal seekblessings", looking for the
	// URL that can be used to retrieve the blessings.
	output := httpGet(t, line)
	if want := "Received blessings: <tt>root:u:testemail@example.com:test-extension"; !strings.Contains(output, want) {
		t.Fatalf("failed to seek blessings: %v", output)
	}
}

func testOauthHandler(t *testing.T, addr string) {
	p := testutil.NewPrincipal("foo")
	keyBytes, err := p.PublicKey().MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	url := addr + "/auth/google/bless?token=tok&output_format=base64vom&public_key=" + base64.URLEncoding.EncodeToString(keyBytes)
	output := httpGet(t, url)
	var blessings security.Blessings
	if raw, err := base64.URLEncoding.DecodeString(output); err != nil {
		t.Fatal(err)
	} else if err = vom.Decode(raw, &blessings); err != nil {
		t.Fatal(err)
	}
	expected := "root:o:test-client-id:testemail@example.com"
	if !blessings.CouldHaveNames([]string{expected}) {
		t.Fatalf("Wanted blessing root:o:clientid:email, got %v", blessings)
	}
}

func readCounter(t *testing.T, ctx *context.T, addr, stat string) (int64, error) {
	v, err := stats.StatsClient(naming.Join(addr, stat)).Value(ctx)
	if err != nil {
		return 0, err
	}
	var c int64
	err = v.ToValue(&c)
	return c, err
}

func TestV23IdentityServer(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Start identityd:
	//
	// identityd must have credentials that recognize the root mounttable.
	// In production, the two share a common root certificate and thus
	// recognize each other. The same is done here: sh.Credentials.Principal
	// wields the root key.
	identityd := v23test.BuildGoPkg(sh, "v.io/x/ref/services/identity/internal/identityd_test")
	creds := sh.ForkCredentials("u")
	oauthCreds := sh.ForkCredentials("o")

	cmd := sh.Cmd(identityd,
		"-v23.tcp.address=127.0.0.1:0",
		"-http-addr=127.0.0.1:0",
		"-oauth-credentials-dir="+oauthCreds.Handle).WithCredentials(creds)
	cmd.Start()
	httpAddr := cmd.S.ExpectVar("HTTP_ADDR")
	v23Addr := cmd.S.ExpectVar("NAME")

	// Use the principal tool to seekblessings.
	// This tool will not run with any credentials: Its whole purpose is to "seek" them!
	principal := v23test.BuildGoPkg(sh, "v.io/x/ref/cmd/principal")
	// Test an initial seekblessings call.
	seekBlessings(t, sh, principal, httpAddr)
	// Test that a subsequent call succeeds with the same
	// credentials. This means that the blessings and principal from the
	// first call works correctly.
	// TODO(ashankar): Does anyone recall what was the intent here? Running
	// the tool twice doesn't seem to help?
	seekBlessings(t, sh, principal, httpAddr)

	// Check stats: Since not blessings were issued over HTTP right now, the value should be 0.
	if got, _ := readCounter(t, sh.ForkContext("u"), v23Addr, "__debug/stats/http/blessings/root:o"); got != 0 {
		t.Errorf("Got %d, want 0 for the count of blessings issued over HTTP", got)
	}
	testOauthHandler(t, httpAddr)
	if got, err := readCounter(t, sh.ForkContext("u"), v23Addr, "__debug/stats/http/blessings/root:o"); got != 1 || err != nil {
		t.Errorf("Got (%d, %v), want (1, <nil>) for the count of blessings issued over HTTP", got, err)
	}
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
