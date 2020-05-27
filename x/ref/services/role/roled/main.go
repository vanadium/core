// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/internal/restsigner"
	irole "v.io/x/ref/services/role/roled/internal"
)

var (
	configDir             string
	name                  string
	remoteSignerBlessings string
)

func main() {
	cmdRoleD.Flags.StringVar(&configDir, "config-dir", "", "The directory where the role configuration files are stored.")
	cmdRoleD.Flags.StringVar(&name, "name", "", "The name to publish for this service.")
	cmdRoleD.Flags.StringVar(&remoteSignerBlessings, "remote-signer-blessings", "", "Path to a file containing base64url-vom-encoded blessings to be used with a remote signer. Empty string disables the remote signer.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoleD)
}

var cmdRoleD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runRoleD),
	Name:   "roled",
	Short:  "Runs the Role interface daemon.",
	Long:   "Command roled runs the Role interface daemon.",
}

func runRoleD(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(configDir) == 0 {
		return env.UsageErrorf("-config-dir must be specified")
	}
	if len(name) == 0 {
		return env.UsageErrorf("-name must be specified")
	}
	if remoteSignerBlessings != "" {
		// TODO(ashankar): Remove this block
		signer, err := restsigner.NewRestSigner()
		if err != nil {
			return fmt.Errorf("failed to create remote signer: %v", err)
		}
		// Decode the blessings and ensure that they are attached to the same public key.
		encoded, err := ioutil.ReadFile(remoteSignerBlessings)
		if err != nil {
			return fmt.Errorf("unable to read --remote-signer-blessings (%s): %v", remoteSignerBlessings, err)
		}
		buf := bytes.NewBuffer(encoded)
		var b security.Blessings
		if err := vom.NewDecoder(base64.NewDecoder(base64.URLEncoding, buf)).Decode(&b); err != nil {
			return fmt.Errorf("unable to decode --remote-signer-blessings (%s): %v", remoteSignerBlessings, err)
		}
		if buf.Len() != 0 {
			return fmt.Errorf("unexpected trailing %d bytes in %s", buf.Len(), remoteSignerBlessings)
		}
		p, err := security.CreatePrincipal(signer, vsecurity.FixedBlessingsStore(b, nil), vsecurity.NewBlessingRoots())
		if err != nil {
			return err
		}
		if err := security.AddToRoots(p, b); err != nil {
			return err
		}
		if ctx, err = v23.WithPrincipal(ctx, p); err != nil {
			return err
		}
	}
	ctx, _, err := v23.WithNewDispatchingServer(ctx, name, irole.NewDispatcher(configDir, name))
	if err != nil {
		return fmt.Errorf("NewServer failed: %v", err)
	}
	fmt.Printf("NAME=%s\n", name)
	<-signals.ShutdownOnSignals(ctx)
	return nil
}
