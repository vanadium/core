// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Daemon groupsd implements the v.io/v23/services/groups interfaces for
// managing access control groups.
package main

// Example invocation:
// groupsd --v23.tcp.address="127.0.0.1:0" --name=groupsd

import (
	"fmt"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/groups/lib"
)

var (
	flagName    string
	flagEngine  string
	flagRootDir string
)

func main() {
	cmdGroupsD.Flags.StringVar(&flagName, "name", "", "Name to mount the groups server as.")
	cmdGroupsD.Flags.StringVar(&flagEngine, "engine", "memstore", "Storage engine to use. Currently supported: leveldb, and memstore.")
	cmdGroupsD.Flags.StringVar(&flagRootDir, "root-dir", "/var/lib/groupsd", "Root dir for storage engines and other data.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdGroupsD)
}

var cmdGroupsD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runGroupsD),
	Name:   "groupsd",
	Short:  "Runs the groups daemon.",
	Long: `
Command groupsd runs the groups daemon, which implements the
v.io/v23/services/groups.Group interface.
`,
}

func runGroupsD(ctx *context.T, env *cmdline.Env, args []string) error {
	ctx, handler := signals.ShutdownOnSignalsWithCancel(ctx)
	defer handler.WaitForSignal()
	dispatcher, err := lib.NewGroupsDispatcher(flagRootDir, flagEngine)
	if err != nil {
		return err
	}
	ctx, server, err := v23.WithNewDispatchingServer(ctx, flagName, dispatcher)
	if err != nil {
		return fmt.Errorf("NewDispatchingServer(%v) failed: %v", flagName, err)
	}
	ctx.Infof("Groups server running at endpoint=%q", server.Status().Endpoints[0].Name())
	return nil
}
