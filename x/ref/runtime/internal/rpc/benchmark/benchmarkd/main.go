// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $GOPATH/src/v.io/vendor/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"v.io/x/lib/cmdline"

	"v.io/v23/context"

	"v.io/v23"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/runtime/internal/rpc/benchmark/internal"
)

func main() {
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

var cmdRoot = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runBenchmarkD),
	Name:   "benchmarkd",
	Short:  "Run the benchmark server",
	Long:   "Command benchmarkd runs the benchmark server.",
}

func runBenchmarkD(ctx *context.T, env *cmdline.Env, args []string) error {
	ctx, server, err := v23.WithNewServer(ctx, "", internal.NewService(), securityflag.NewAuthorizerOrDie())
	if err != nil {
		ctx.Fatalf("NewServer failed: %v", err)
	}
	ctx.Infof("Listening on %s", server.Status().Endpoints[0].Name())
	<-signals.ShutdownOnSignals(ctx)
	return nil
}
