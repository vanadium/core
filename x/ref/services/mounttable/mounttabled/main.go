// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/mounttable/mounttablelib"
)

var opts = mounttablelib.Opts{}

func main() {
	opts.InitFlags(&cmd.Flags)
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmd)
}

var cmd = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(run),
	Name:   "mounttabled",
	Short:  "Runs the mount table daemon",
	Long: `
Command mounttabled runs the mount table daemon, which implements the
v.io/v23/services/mounttable interfaces.
`,
}

func run(ctx *context.T, env *cmdline.Env, args []string) error {
	return mounttablelib.MainWithCtx(ctx, opts)
}
