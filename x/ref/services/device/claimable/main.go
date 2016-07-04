// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/device/internal/claim"
)

var (
	permsDir      string
	rootBlessings string
)

func runServer(ctx *context.T, _ *cmdline.Env, _ []string) error {
	if rootBlessings != "" {
		addRoot(ctx, rootBlessings)
	}

	auth := securityflag.NewAuthorizerOrDie()
	claimable, claimed := claim.NewClaimableDispatcher(ctx, permsDir, "", auth)
	if claimable == nil {
		return errors.New("device is already claimed")
	}

	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", claimable)
	if err != nil {
		return err
	}

	status := server.Status()
	ctx.Infof("Listening on: %v", status.Endpoints)
	if len(status.Endpoints) > 0 {
		fmt.Printf("NAME=%s\n", status.Endpoints[0].Name())
	}
	select {
	case <-claimed:
		return nil
	case s := <-signals.ShutdownOnSignals(ctx):
		return fmt.Errorf("received signal %v", s)
	}
}

func addRoot(ctx *context.T, flagRoots string) {
	p := v23.GetPrincipal(ctx)
	for _, b64 := range strings.Split(flagRoots, ",") {
		// We use URLEncoding to be compatible with the principal
		// command.
		vomBlessings, err := base64.URLEncoding.DecodeString(b64)
		if err != nil {
			ctx.Fatalf("unable to decode the base64 blessing roots: %v", err)
		}
		var blessings security.Blessings
		if err := vom.Decode(vomBlessings, &blessings); err != nil {
			ctx.Fatalf("unable to decode the vom blessing roots: %v", err)
		}
		if err := security.AddToRoots(p, blessings); err != nil {
			ctx.Fatalf("unable to add blessing roots: %v", err)
		}
	}
}

func main() {
	rootCmd := &cmdline.Command{
		Name:  "claimable",
		Short: "Run claimable server",
		Long: `
Claimable is a server that implements the Claimable interface from
v.io/v23/services/device. It exits immediately if the device is already
claimed. Otherwise, it keeps running until a successful Claim() request
is received.

It uses -v23.permissions.* to authorize the Claim request.
`,
		Runner: v23cmd.RunnerFunc(runServer),
	}
	rootCmd.Flags.StringVar(&permsDir, "perms-dir", "", "The directory where permissions will be stored.")
	rootCmd.Flags.StringVar(&rootBlessings, "root-blessings", "", "A comma-separated list of the root blessings to trust, base64-encoded VOM-encoded.")
	cmdline.Main(rootCmd)
}
