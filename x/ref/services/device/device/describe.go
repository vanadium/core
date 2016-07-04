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

var cmdDescribe = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runDescribe),
	Name:     "describe",
	Short:    "Describe the device.",
	Long:     "Describe the device.",
	ArgsName: "<device>",
	ArgsLong: `
<device> is the vanadium object name of the device manager's device service.`,
}

func runDescribe(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("describe: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	deviceName := args[0]
	if description, err := device.DeviceClient(deviceName).Describe(ctx); err != nil {
		return fmt.Errorf("Describe failed: %v", err)
	} else {
		fmt.Fprintf(env.Stdout, "%+v\n", description)
	}
	return nil
}
