// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/identity/internal/auditor"
	"v.io/x/ref/services/identity/internal/caveats"
	"v.io/x/ref/services/identity/internal/handlers"
	"v.io/x/ref/services/identity/internal/oauth"
	"v.io/x/ref/services/identity/internal/revocation"
	"v.io/x/ref/services/identity/internal/server"
	"v.io/x/ref/services/internal/dbutil"
	"v.io/x/ref/services/internal/restsigner"
)

var (
	googleConfigWeb                                                  string
	externalHTTPAddr, httpAddr, tlsConfig, assetsPrefix, mountPrefix string
	dischargerLocation                                               string
	sqlConf                                                          string
	registeredAppConfig                                              string
	userBlessings, appBlessings                                      string
)

func init() {
	// Configuration for various Google OAuth-based clients.
	cmdIdentityD.Flags.StringVar(&googleConfigWeb, "google-config-web", "", "Path to JSON-encoded OAuth client configuration for the web application that renders the audit log for blessings provided by this provider.")

	// Configuration using the remote signer
	cmdIdentityD.Flags.StringVar(&userBlessings, "user-blessings", "", "Path to a file containing base64url-vom encoded blessings that will be extended with the username of the requestor.")
	cmdIdentityD.Flags.StringVar(&appBlessings, "app-blessings", "", "Path to a file containing base64url-vom encoded blessings that will be extended with an application identifier and the username of the requestor (i.e., a user using a specific app)")
	cmdIdentityD.Flags.StringVar(&registeredAppConfig, "registered-apps", "", "Path to the config file for registered oauth clients.")

	// Flags controlling the HTTP server
	cmdIdentityD.Flags.StringVar(&externalHTTPAddr, "external-http-addr", "", "External address on which the HTTP server listens on.  If none is provided the server will only listen on -http-addr.")
	cmdIdentityD.Flags.StringVar(&httpAddr, "http-addr", "localhost:8125", "Address on which the HTTP server listens on.")
	cmdIdentityD.Flags.StringVar(&tlsConfig, "tls-config", "", "Comma-separated list of TLS certificate and private key files, in that order.  This must be provided.")
	cmdIdentityD.Flags.StringVar(&assetsPrefix, "assets-prefix", "", "Host serving the web assets for the identity server.")
	cmdIdentityD.Flags.StringVar(&mountPrefix, "mount-prefix", "identity", "Mount name prefix to use.  May be rooted.")

	// Flag controlling auditing and revocation of Blessing operations
	cmdIdentityD.Flags.StringVar(&sqlConf, "sql-config", "", "Path to configuration file for MySQL database connection. Database is used to persist blessings for auditing and revocation. "+dbutil.SQLConfigFileDescription)
	cmdIdentityD.Flags.StringVar(&dischargerLocation, "discharger-location", "", "The name of the discharger service. May be rooted. If empty, the published name is used.")
}

func main() {
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdIdentityD)
}

var cmdIdentityD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runIdentityD),
	Name:   "identityd",
	Short:  "Runs HTTP server that creates security.Blessings objects",
	Long: `
Command identityd runs a daemon HTTP server that uses OAuth to create
security.Blessings objects.

Starts an HTTP server that brokers blessings after authenticating through OAuth.

To generate TLS certificates so the HTTP server can use SSL:
  go run $(go list -f {{.Dir}} "crypto/tls")/generate_cert.go --host <IP address>

To use Google as an OAuth provider the -google-config-web flag must be set to
point to the a JSON file obtained after registering the application with the
Google Developer Console at https://cloud.google.com/console

More details on Google OAuth at:
  https://developers.google.com/accounts/docs/OAuth2Login

More details on the design of identityd at:
  https://vanadium.github.io/designdocs/identity-service.html
`,
}

func runIdentityD(ctx *context.T, env *cmdline.Env, args []string) error {
	var sqlDB *sql.DB
	var err error
	if sqlConf != "" {
		if sqlDB, err = dbutil.NewSQLDBConnFromFile(sqlConf, "SERIALIZABLE"); err != nil {
			return env.UsageErrorf("Failed to create sqlDB: %v", err)
		}
	}

	if ctx, err = initRemoteSigner(ctx, userBlessings); err != nil {
		return err
	}

	oauthCtx, err := initRemoteSigner(ctx, appBlessings)
	if err != nil {
		return err
	}

	registeredApps, err := readRegisteredAppsConfig()
	if err != nil {
		return err
	}

	googleoauth, err := oauth.NewGoogleOAuth(ctx, googleConfigWeb)
	if err != nil {
		return env.UsageErrorf("Failed to setup GoogleOAuth: %v", err)
	}

	auditor, reader, err := auditor.NewSQLBlessingAuditor(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("Failed to create sql auditor from config: %v", err)
	}

	revocationManager, err := revocation.NewRevocationManager(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("Failed to start RevocationManager: %v", err)
	}

	s := server.NewIdentityServer(
		googleoauth,
		auditor,
		reader,
		revocationManager,
		caveats.NewBrowserCaveatSelector(assetsPrefix),
		assetsPrefix,
		mountPrefix,
		dischargerLocation,
		registeredApps)
	s.Serve(ctx, oauthCtx, externalHTTPAddr, httpAddr, tlsConfig)
	return nil
}

func initRemoteSigner(ctx *context.T, blessings string) (*context.T, error) {
	if len(blessings) == 0 {
		return ctx, nil
	}
	signer, err := restsigner.NewRestSigner()
	if err != nil {
		return nil, err
	}
	// Decode the blessings and ensure that they are attached to the same public key.
	encoded, err := ioutil.ReadFile(blessings)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(encoded)
	var b security.Blessings
	if err := vom.NewDecoder(base64.NewDecoder(base64.URLEncoding, buf)).Decode(&b); err != nil {
		return nil, err
	}
	if buf.Len() != 0 {
		return nil, fmt.Errorf("unexpected trailing %d bytes in %s", buf.Len(), blessings)
	}
	p, err := security.CreatePrincipal(signer, vsecurity.FixedBlessingsStore(b, nil), vsecurity.NewBlessingRoots())
	if err != nil {
		return nil, err
	}
	if err := security.AddToRoots(p, b); err != nil {
		return nil, err
	}
	return v23.WithPrincipal(ctx, p)
}

func readRegisteredAppsConfig() (registeredApps handlers.RegisteredAppMap, err error) {
	if registeredAppConfig == "" {
		return nil, nil
	}
	var configFile *os.File
	configFile, err = os.Open(registeredAppConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to open registered app config: %v", err)
	}
	defer configFile.Close()
	dec := json.NewDecoder(configFile)
	if err = dec.Decode(&registeredApps); err != nil {
		return nil, fmt.Errorf("Error parsing registered apps: %v", err)

	}
	return registeredApps, nil
}
