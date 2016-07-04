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

var cmdList = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runList),
	Name:     "list",
	Short:    "Lists the account associations.",
	Long:     "Lists all account associations.",
	ArgsName: "<devicemanager>.",
	ArgsLong: `
<devicemanager> is the name of the device manager to connect to.`,
}

func runList(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("list: incorrect number of arguments, expected %d, got %d", expected, got)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	assocs, err := device.DeviceClient(args[0]).ListAssociations(ctx)
	if err != nil {
		return fmt.Errorf("ListAssociations failed: %v", err)
	}

	for _, a := range assocs {
		fmt.Fprintf(env.Stdout, "%s %s\n", a.IdentityName, a.AccountName)
	}
	return nil
}

var cmdAdd = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runAdd),
	Name:     "add",
	Short:    "Add the listed blessings with the specified system account.",
	Long:     "Add the listed blessings with the specified system account.",
	ArgsName: "<devicemanager> <systemName> <blessing>...",
	ArgsLong: `
<devicemanager> is the name of the device manager to connect to.
<systemName> is the name of an account holder on the local system.
<blessing>.. are the blessings to associate systemAccount with.`,
}

func runAdd(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 3, len(args); got < expected {
		return env.UsageErrorf("add: incorrect number of arguments, expected at least %d, got %d", expected, got)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	return device.DeviceClient(args[0]).AssociateAccount(ctx, args[2:], args[1])
}

var cmdRemove = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runRemove),
	Name:     "remove",
	Short:    "Removes system accounts associated with the listed blessings.",
	Long:     "Removes system accounts associated with the listed blessings.",
	ArgsName: "<devicemanager>  <blessing>...",
	ArgsLong: `
<devicemanager> is the name of the device manager to connect to.
<blessing>... is a list of blessings.`,
}

func runRemove(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 2, len(args); got < expected {
		return env.UsageErrorf("remove: incorrect number of arguments, expected at least %d, got %d", expected, got)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	return device.DeviceClient(args[0]).AssociateAccount(ctx, args[1:], "")
}

var cmdAssociate = &cmdline.Command{
	Name:  "associate",
	Short: "Tool for creating associations between Vanadium blessings and a system account",
	Long: `
The associate tool facilitates managing blessing to system account associations.
`,
	Children: []*cmdline.Command{cmdList, cmdAdd, cmdRemove},
}
