// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/runtime/internal/rpc/stress"
)

var cmdStopServers = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runStopServers),
	Name:     "stop",
	Short:    "Stop servers",
	Long:     "Stop servers",
	ArgsName: "<server> ...",
	ArgsLong: "<server> ... A list of servers to stop.",
}

func runStopServers(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		return env.UsageErrorf("no server specified")
	}
	for _, server := range args {
		if err := stress.StressClient(server).Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	cmdRoot := &cmdline.Command{
		Name:  "stress",
		Short: "Tool to stress/load test RPC",
		Long:  "Command stress is a tool to stress/load test RPC by issuing randomly generated requests.",
		Children: []*cmdline.Command{
			cmdStressTest,
			cmdStressStats,
			cmdLoadTest,
			cmdStopServers,
		},
	}
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}
