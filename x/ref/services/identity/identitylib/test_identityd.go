// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package identitylib implements a test identityd service registered using the
// v.io/x/lib/gosh library.
package identitylib

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"time"

	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/services/identity/internal/auditor"
	"v.io/x/ref/services/identity/internal/caveats"
	"v.io/x/ref/services/identity/internal/oauth"
	"v.io/x/ref/services/identity/internal/revocation"
	"v.io/x/ref/services/identity/internal/server"
	"v.io/x/ref/services/identity/internal/util"
	"v.io/x/ref/test"
)

var (
	externalHTTPAddr = flag.String("external-http-addr", "", "External address on which the HTTP server listens on. If none is provided the server will only listen on -http-addr.")
	httpAddr         = flag.CommandLine.String("http-addr", "localhost:0", "Address on which the HTTP server listens on.")
	tlsConfig        = flag.CommandLine.String("tls-config", "", "Comma-separated list of TLS certificate and private key files. This must be provided.")
)

var TestIdentityd = gosh.RegisterFunc("TestIdentityd", func() error {
	// Duration to use for tls cert and blessing duration.
	duration := 365 * 24 * time.Hour

	ctx, shutdown := test.V23Init()
	defer shutdown()

	// If no tls-config has been provided, generate new cert and key and use them.
	if flag.CommandLine.Lookup("tls-config").Value.String() == "" {
		addr := *externalHTTPAddr
		if *externalHTTPAddr == "" {
			addr = *httpAddr
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("Failed to parse %q: %v", addr, err)
		}
		certFile, keyFile, err := util.WriteCertAndKey(host, duration)
		if err != nil {
			return fmt.Errorf("Could not write cert and key: %v", err)
		}
		if err := flag.CommandLine.Set("tls-config", certFile+","+keyFile); err != nil {
			return fmt.Errorf("Could not set tls-config: %v", err)
		}
	}

	// Pick a free port if http-addr flag is not set.
	// We can't use :0 here, because the identity server calls
	// http.ListenAndServeTLS, which blocks, leaving us with no way to tell
	// what port the server is running on.  Hence, we must pass in an
	// actual port so we know where the server is running.
	if flag.CommandLine.Lookup("http-addr").Value.String() == flag.CommandLine.Lookup("http-addr").DefValue {
		if err := flag.CommandLine.Set("http-addr", "localhost:"+freePort()); err != nil {
			return fmt.Errorf("Could not set http-addr: %v", err)
		}
	}

	auditor, reader := auditor.NewMockBlessingAuditor()
	revocationManager := revocation.NewMockRevocationManager(ctx)
	oauthProvider := oauth.NewMockOAuth("testemail@example.com", "test-client-id")

	s := server.NewIdentityServer(
		oauthProvider,
		auditor,
		reader,
		revocationManager,
		caveats.NewMockCaveatSelector(),
		"",
		"identity",
		"",
		nil)

	_, eps, externalHTTPAddress := s.Listen(ctx, ctx, *externalHTTPAddr, *httpAddr, *tlsConfig)

	fmt.Printf("TEST_IDENTITYD_NAME=%s\n", eps[0])
	fmt.Printf("TEST_IDENTITYD_HTTP_ADDR=%s\n", externalHTTPAddress)

	<-signals.ShutdownOnSignals(ctx)
	return nil
})

func freePort() string {
	l, _ := net.Listen("tcp", ":0")
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}
