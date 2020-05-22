// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/runtime/internal/rpc/stress/internal"
)

var duration time.Duration
var name string

func main() {
	cmdRoot.Flags.DurationVar(&duration, "duration", 0, "Duration of the stress test to run; if zero, there is no limit.")
	cmdRoot.Flags.StringVar(&name, "name", "", "Name to mount the server under.  If empty, don't mount.")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

var cmdRoot = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runStressD),
	Name:   "stressd",
	Short:  "Run the stress-test server",
	Long:   "Command stressd runs the stress-test server.",
}

func runStressD(ctx *context.T, env *cmdline.Env, args []string) error {
	service, stop := internal.NewService()
	ctx, server, err := v23.WithNewServer(ctx, name, service, security.AllowEveryone())
	if err != nil {
		ctx.Fatalf("NewServer failed: %v", err)
	}
	ctx.Infof("listening on %s", server.Status().Endpoints[0].Name())

	var timeout <-chan time.Time
	if duration > 0 {
		timeout = time.After(duration)
	}
	select {
	case <-timeout:
	case <-stop:
	case <-signals.ShutdownOnSignals(ctx):
	}
	return nil
}
