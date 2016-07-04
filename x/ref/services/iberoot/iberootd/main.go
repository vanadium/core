// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"fmt"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"

	"v.io/x/lib/cmdline"
	"v.io/x/lib/ibe"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	iroot "v.io/x/ref/services/iberoot/iberootd/internal"
)

var (
	genmode bool
	keyDir  string
	objName string
	idpName string
)

func main() {
	cmdRootD.Flags.StringVar(&keyDir, "key-dir", "", "The directory where the master private key and params are stored.")
	cmdRootD.Flags.BoolVar(&genmode, "generate", false, "Generate fresh master private key and params, save them in the directory specified by --key-dir and exit")
	// TODO(ataly): We could also obtain this name from the default
	// blessing of this service. Unfortuantely that gets messy when
	// there are multiple default blessings, and it forces starting
	// this service under the same set of blessings as the identity
	// provider. Therefore for now we keep the mapping between this
	// service and the IDP explicit by taking in the IDP's blessing
	// name as a flag. This TODO is for revisiting this choice in the
	// future.
	cmdRootD.Flags.StringVar(&idpName, "idp", "", "The name of the identity provider represented by this service.")
	cmdRootD.Flags.StringVar(&objName, "name", "", "The name to publish for this service.")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRootD)
}

var cmdRootD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runRootD),
	Name:   "iberootd",
	Short:  "Runs the bcrypter.Root interface daemon.",
	Long:   "Command iberootd runs the bcrypter.Root interface daemon.",
}

func runRootD(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(keyDir) == 0 {
		return env.UsageErrorf("-key-dir must be specified")
	}
	if genmode {
		master, err := ibe.SetupBB2()
		if err != nil {
			return fmt.Errorf("ibe.SetupBB2() failed: %v", err)
		}
		if err := iroot.SaveMaster(master, keyDir); err != nil {
			return fmt.Errorf("SaveMaster failed: %v", err)
		}
		return nil
	}
	if len(objName) == 0 {
		return env.UsageErrorf("-name must be specified")
	}
	if len(idpName) == 0 {
		return env.UsageErrorf("-idp must be specified")
	}
	root, err := iroot.NewRootServer(keyDir, idpName)
	if err != nil {
		return fmt.Errorf("Cannot create iberoot.Root object: %v", err)
	}
	ctx, server, err := v23.WithNewServer(ctx, objName, root, security.AllowEveryone())
	if err != nil {
		return fmt.Errorf("WithNewServer failed: %v", err)
	}
	fmt.Println("ENDPOINT=", server.Status().Endpoints[0].Name())
	<-signals.ShutdownOnSignals(ctx)
	return nil
}
