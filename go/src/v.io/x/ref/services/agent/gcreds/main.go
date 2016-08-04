// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	goauth2 "google.golang.org/api/oauth2/v2"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline"
	"v.io/x/ref"
	lsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/services/agent/internal/ipc"
	"v.io/x/ref/services/agent/internal/server"

	_ "v.io/x/ref/runtime/factories/roaming"
)

var (
	keyFileFlag      string
	oauthBlesserFlag string
)

func main() {
	cmdGcreds.Flags.StringVar(&keyFileFlag, "key-file", "", "The JSON file containing the Google Cloud credentials to use. If empty, the default credentials will be used.")
	cmdGcreds.Flags.StringVar(&oauthBlesserFlag, "oauth-blesser", "https://dev.v.io/auth/google/bless", "The URL of the OAuthBlesser service.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdGcreds)
}

var cmdGcreds = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runGcreds),
	Name:   "gcreds",
	Short:  "Runs a command with Google Cloud blessings",
	Long: `
Command gcreds runs a command with Google Cloud Blessings.

The Google Cloud credentials can be specified with the --key-file flag. If
--key-file is not set, the default credentials will be used. These credentials
are exchanged for Vanadium blessings using the OAuthBlesser service specified
with the --oauth-blesser flag.

The command is executed with an ephemeral principal that has the blessings
received from the OAuthBlesser service. The principal and its blessings are held
in memory and do not persist after gcreds exits.

See https://developers.google.com/identity/protocols/application-default-credentials
for more information on Google Cloud credentials.
`,
	ArgsName: "<command>",
}

func runGcreds(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		return env.UsageErrorf("missing command")
	}
	p, err := lsecurity.NewPrincipal()
	if err != nil {
		return err
	}
	if ctx, err = v23.WithPrincipal(ctx, p); err != nil {
		return err
	}
	if err = getGoogleCloudBlessings(ctx); err != nil {
		return err
	}
	// Refresh the blessings periodically.
	go refreshCreds(ctx)

	workDir, err := ioutil.TempDir("", "gcreds-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	socketPath := filepath.Join(workDir, "agent.sock")

	// Run the server.
	i := ipc.NewIPC()
	defer i.Close()
	if err = server.ServeAgent(i, p); err != nil {
		return err
	}

	if err = i.Listen(socketPath); err != nil {
		return err
	}

	ref.EnvClearCredentials()
	if err := os.Setenv(ref.EnvAgentPath, socketPath); err != nil {
		return err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = env.Stdin
	cmd.Stdout = env.Stdout
	cmd.Stderr = env.Stderr
	return cmd.Run()
}

func refreshCreds(ctx *context.T) {
	for {
		var (
			p    = v23.GetPrincipal(ctx)
			b, _ = p.BlessingStore().Default()
			exp  = b.Expiry()
		)
		if exp.IsZero() {
			// Nothing to do.
			return
		}
		delay := exp.Add(-time.Minute).Sub(time.Now())
		if delay < time.Second {
			delay = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			ctx.Info("refreshing credentials")
			if err := getGoogleCloudBlessings(ctx); err != nil {
				ctx.Errorf("getGoogleCloudBlessings: %v", err)
			}
		}
	}
}

// getGoogleCloudBlessings gets blessings using Google Cloud OAuth credentials.
func getGoogleCloudBlessings(ctx *context.T) error {
	const scope = goauth2.UserinfoEmailScope
	var tokSource oauth2.TokenSource
	if len(keyFileFlag) > 0 {
		data, err := ioutil.ReadFile(keyFileFlag)
		if err != nil {
			return err
		}
		conf, err := google.JWTConfigFromJSON(data, scope)
		if err != nil {
			return err
		}
		tokSource = conf.TokenSource(oauth2.NoContext)
	} else {
		var err error
		if tokSource, err = google.DefaultTokenSource(oauth2.NoContext, scope); err != nil {
			return err
		}
	}
	token, err := tokSource.Token()
	if err != nil {
		return err
	}

	principal := v23.GetPrincipal(ctx)
	bytes, err := principal.PublicKey().MarshalBinary()
	if err != nil {
		return err
	}
	expiry, err := security.NewExpiryCaveat(time.Now().Add(10 * time.Minute))
	if err != nil {
		return err
	}
	caveats, err := base64VomEncode([]security.Caveat{expiry})
	if err != nil {
		return err
	}
	// This interface is defined in:
	// https://godoc.org/v.io/x/ref/services/identity/internal/handlers#NewOAuthBlessingHandler
	v := url.Values{
		"public_key":    {base64.URLEncoding.EncodeToString(bytes)},
		"token":         {token.AccessToken},
		"caveats":       {caveats},
		"output_format": {"base64vom"},
	}
	for attempt := 0; attempt < 30; attempt++ {
		if attempt > 0 {
			ctx.Infof("retrying")
			time.Sleep(time.Second)
		}
		if body, err := postBlessRequest(v); err == nil {
			var blessings security.Blessings
			if err := base64VomDecode(string(body), &blessings); err != nil {
				return err
			}
			return lsecurity.SetDefaultBlessings(principal, blessings)
		} else {
			ctx.Infof("error from oauth-blesser: %v", err)
		}
	}
	return fmt.Errorf("too many failures")
}

func postBlessRequest(values url.Values) ([]byte, error) {
	resp, err := http.PostForm(oauthBlesserFlag, values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got %s", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func base64VomEncode(i interface{}) (string, error) {
	v, err := vom.Encode(i)
	return base64.URLEncoding.EncodeToString(v), err
}

func base64VomDecode(s string, i interface{}) error {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return vom.Decode(b, i)
}
