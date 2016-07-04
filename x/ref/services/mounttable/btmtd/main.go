// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/options"

	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/mounttable/btmtd/internal"
)

var (
	cmdRoot = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runMT),
		Name:   "btmtd",
		Short:  "Runs the mounttable service",
		Long:   "Runs the mounttable service.",
		Children: []*cmdline.Command{
			cmdSetup, cmdDestroy, cmdDump, cmdFsck,
		},
	}
	cmdSetup = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runSetup),
		Name:   "setup",
		Short:  "Creates and sets up the table",
		Long:   "Creates and sets up the table.",
	}
	cmdDestroy = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runDestroy),
		Name:   "destroy",
		Short:  "Destroy the table",
		Long:   "Destroy the table. All data will be lost.",
	}
	cmdDump = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runDump),
		Name:   "dump",
		Short:  "Dump the table",
		Long:   "Dump the table.",
	}
	cmdFsck = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runFsck),
		Name:   "fsck",
		Short:  "Check the table consistency",
		Long:   "Check the table consistency.",
	}

	keyFileFlag      string
	projectFlag      string
	zoneFlag         string
	clusterFlag      string
	tableFlag        string
	inMemoryTestFlag bool

	permissionsFileFlag string
	mountNameFlag       string

	maxNodesPerUserFlag   int
	maxServersPerUserFlag int

	fixFlag bool
)

func main() {
	rand.Seed(time.Now().UnixNano())
	cmdRoot.Flags.StringVar(&keyFileFlag, "key-file", "", "The file that contains the Google Cloud JSON credentials to use")
	cmdRoot.Flags.StringVar(&projectFlag, "project", "", "The Google Cloud project of the Cloud Bigtable cluster")
	cmdRoot.Flags.StringVar(&zoneFlag, "zone", "", "The Google Cloud zone of the Cloud Bigtable cluster")
	cmdRoot.Flags.StringVar(&clusterFlag, "cluster", "", "The Cloud Bigtable cluster name")
	cmdRoot.Flags.StringVar(&tableFlag, "table", "mounttable", "The name of the table to use")
	cmdRoot.Flags.BoolVar(&inMemoryTestFlag, "in-memory-test", false, "If true, use an in-memory bigtable server (for testing only)")
	cmdRoot.Flags.StringVar(&permissionsFileFlag, "permissions-file", "", "The file that contains the initial node permissions.")
	cmdRoot.Flags.StringVar(&mountNameFlag, "name", "", "If provided, causes the mount table to mount itself under this name.")

	cmdRoot.Flags.IntVar(&maxNodesPerUserFlag, "max-nodes-per-user", 10000, "The maximum number of nodes that a single user can create.")
	cmdRoot.Flags.IntVar(&maxServersPerUserFlag, "max-servers-per-user", 10000, "The maximum number of servers that a single user can mount.")

	cmdFsck.Flags.BoolVar(&fixFlag, "fix", false, "Whether to fix consistency errors and recreate all the counters.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

func runSetup(ctx *context.T, env *cmdline.Env, args []string) error {
	bt, err := internal.NewBigTable(keyFileFlag, projectFlag, zoneFlag, clusterFlag, tableFlag)
	if err != nil {
		return err
	}
	return bt.SetupTable(ctx, permissionsFileFlag)
}

func runDestroy(ctx *context.T, env *cmdline.Env, args []string) error {
	bt, err := internal.NewBigTable(keyFileFlag, projectFlag, zoneFlag, clusterFlag, tableFlag)
	if err != nil {
		return err
	}
	return bt.DeleteTable(ctx)
}

func runDump(ctx *context.T, env *cmdline.Env, args []string) error {
	bt, err := internal.NewBigTable(keyFileFlag, projectFlag, zoneFlag, clusterFlag, tableFlag)
	if err != nil {
		return err
	}
	return bt.DumpTable(ctx)
}

func runFsck(ctx *context.T, env *cmdline.Env, args []string) error {
	bt, err := internal.NewBigTable(keyFileFlag, projectFlag, zoneFlag, clusterFlag, tableFlag)
	if err != nil {
		return err
	}
	if fixFlag {
		fmt.Fprintln(env.Stdout, "WARNING: Make sure nothing else is modifying the table while fsck is running")
		fmt.Fprint(env.Stdout, "Continue [y/N]? ")
		var answer string
		if _, err := fmt.Scanf("%s", &answer); err != nil || strings.ToUpper(answer) != "Y" {
			fmt.Fprintln(env.Stdout, "Aborted")
			return nil
		}
	}
	return bt.Fsck(ctx, fixFlag)
}

func runMT(ctx *context.T, env *cmdline.Env, args []string) error {
	var (
		bt  *internal.BigTable
		err error
	)
	if inMemoryTestFlag {
		var shutdown func()
		if bt, shutdown, err = internal.NewTestBigTable(tableFlag); err != nil {
			return err
		}
		defer shutdown()
		if err := bt.SetupTable(ctx, permissionsFileFlag); err != nil {
			return err
		}
	} else {
		if bt, err = internal.NewBigTable(keyFileFlag, projectFlag, zoneFlag, clusterFlag, tableFlag); err != nil {
			return err
		}
	}

	globalPerms, err := securityflag.PermissionsFromFlag()
	if err != nil {
		return err
	}
	cfg := &internal.Config{
		GlobalAcl:         globalPerms["Admin"],
		MaxNodesPerUser:   int64(maxNodesPerUserFlag),
		MaxServersPerUser: int64(maxServersPerUserFlag),
	}
	disp := internal.NewDispatcher(bt, cfg)
	ctx, server, err := v23.WithNewDispatchingServer(ctx, mountNameFlag, disp, options.ServesMountTable(true), options.LameDuckTimeout(30*time.Second))
	if err != nil {
		return err
	}
	ctx.Infof("Listening on: %v", server.Status().Endpoints)

	<-signals.ShutdownOnSignals(ctx)
	return nil
}
