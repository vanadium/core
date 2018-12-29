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

var cmdUninstall = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runUninstall),
	Name:     "uninstall",
	Short:    "Uninstall the given application installation.",
	Long:     "Uninstall the given application installation.",
	ArgsName: "<installation>",
	ArgsLong: `
<installation> is the vanadium object name of the application installation to
uninstall.
`,
}

func runUninstall(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("uninstall: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	installName := args[0]
	if err := device.ApplicationClient(installName).Uninstall(ctx); err != nil {
		return fmt.Errorf("Uninstall failed: %v", err)
	}
	fmt.Fprintf(env.Stdout, "Successfully uninstalled: %q\n", installName)
	return nil
}
