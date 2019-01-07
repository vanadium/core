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

var cmdDelete = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runDelete),
	Name:     "delete",
	Short:    "Delete the given application instance.",
	Long:     "Delete the given application instance.",
	ArgsName: "<app instance>",
	ArgsLong: `
<app instance> is the vanadium object name of the application instance to delete.`,
}

func runDelete(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("delete: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appName := args[0]

	if err := device.ApplicationClient(appName).Delete(ctx); err != nil {
		return fmt.Errorf("Delete failed: %v", err)
	}
	fmt.Fprintf(env.Stdout, "Delete succeeded\n")
	return nil
}
