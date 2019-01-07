// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"time"

	"v.io/v23/context"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
)

const killDeadline = 10 * time.Second

var cmdKill = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runKill),
	Name:     "kill",
	Short:    "Kill the given application instance.",
	Long:     "Kill the given application instance.",
	ArgsName: "<app instance>",
	ArgsLong: `
<app instance> is the vanadium object name of the application instance to kill.`,
}

func runKill(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("kill: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appName := args[0]

	if err := device.ApplicationClient(appName).Kill(ctx, killDeadline); err != nil {
		return fmt.Errorf("Kill failed: %v", err)
	}
	fmt.Fprintf(env.Stdout, "Kill succeeded\n")
	return nil
}
