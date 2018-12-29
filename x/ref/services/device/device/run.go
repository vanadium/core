// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"v.io/v23/context"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
)

var cmdRun = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runRun),
	Name:     "run",
	Short:    "Run the given application instance.",
	Long:     "Run the given application instance.",
	ArgsName: "<app instance>",
	ArgsLong: `
<app instance> is the vanadium object name of the application instance to run.`,
}

func runRun(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("run: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appName := args[0]

	if err := device.ApplicationClient(appName).Run(ctx); err != nil {
		return fmt.Errorf("Run failed: %v,\nView log with:\n debug logs read `debug glob %s/logs/STDERR-*`", err, appName)
	}
	fmt.Fprintf(env.Stdout, "Run succeeded\n")
	return nil
}
