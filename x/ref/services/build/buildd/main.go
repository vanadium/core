// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"fmt"
	"os"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/services/build"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
)

var gobin, goroot, name string

func main() {
	cmdBuildD.Flags.StringVar(&gobin, "gobin", "go", "Path to the Go compiler.")
	cmdBuildD.Flags.StringVar(&goroot, "goroot", os.Getenv("GOROOT"), "GOROOT to use with the Go compiler.  The default is the value of the GOROOT environment variable.")
	cmdBuildD.Flags.Lookup("goroot").DefValue = "<GOROOT>"
	cmdBuildD.Flags.StringVar(&name, "name", "", "Name to mount the build server as.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdBuildD)
}

var cmdBuildD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runBuildD),
	Name:   "buildd",
	Short:  "Runs the builder daemon.",
	Long: `
Command buildd runs the builder daemon, which implements the
v.io/v23/services/build.Builder interface.
`,
}

func runBuildD(ctx *context.T, env *cmdline.Env, args []string) error {
	ctx, server, err := v23.WithNewServer(ctx, name, build.BuilderServer(NewBuilderService(gobin, goroot)), securityflag.NewAuthorizerOrDie())
	if err != nil {
		return fmt.Errorf("NewServer() failed: %v", err)
	}
	ctx.Infof("Build server running at endpoint=%q", server.Status().Endpoints[0].Name())

	// Wait until shutdown.
	<-signals.ShutdownOnSignals(ctx)
	return nil
}
