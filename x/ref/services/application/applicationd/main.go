// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"fmt"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
)

var name, store string

func main() {
	cmdAppD.Flags.StringVar(&name, "name", "", "Name to mount the application repository as.")
	cmdAppD.Flags.StringVar(&store, "store", "", "Local directory to store application envelopes in.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdAppD)
}

var cmdAppD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runAppD),
	Name:   "applicationd",
	Short:  "Runs the application daemon.",
	Long: `
Command applicationd runs the application daemon, which implements the
v.io/x/ref/services/repository.Application interface.
`,
}

func runAppD(ctx *context.T, env *cmdline.Env, args []string) error {
	if store == "" {
		return env.UsageErrorf("Specify a directory for storing application envelopes using --store=<name>")
	}

	dispatcher, err := NewDispatcher(store)
	if err != nil {
		return fmt.Errorf("NewDispatcher() failed: %v", err)
	}

	ctx, server, err := v23.WithNewDispatchingServer(ctx, name, dispatcher)
	if err != nil {
		return fmt.Errorf("NewServer() failed: %v", err)
	}
	epName := server.Status().Endpoints[0].Name()
	if name != "" {
		ctx.Infof("Application repository serving at %q (%q)", name, epName)
	} else {
		ctx.Infof("Application repository serving at %q", epName)
	}
	// Wait until shutdown.
	<-signals.ShutdownOnSignals(ctx)
	return nil
}
