// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"v.io/x/lib/cmdline"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"

	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
)

func main() {
	cmdline.HideGlobalFlagsExcept(regexp.MustCompile(`^v23\.namespace\.root$`))
	cmdline.Main(cmdRoot)
}

var cmdGlob = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runGlob),
	Name:     "glob",
	Short:    "returns all matching entries in the mount table",
	Long:     "returns all matching entries in the mount table",
	ArgsName: "[<mount name>] <pattern>",
	ArgsLong: `
<mount name> is a mount name on a mount table.  Defaults to namespace root.
<pattern> is a glob pattern that is matched against all the entries below the
specified mount name.
`,
}

func runGlob(ctx *context.T, env *cmdline.Env, args []string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	if len(args) == 1 {
		roots := v23.GetNamespace(ctx).Roots()
		if len(roots) == 0 {
			return errors.New("no namespace root")
		}
		args = append([]string{roots[0]}, args...)
	}
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("glob: incorrect number of arguments, expected %d, got %d", expected, got)
	}

	name, pattern := args[0], args[1]
	client := v23.GetClient(ctx)
	call, err := client.StartCall(ctx, name, rpc.GlobMethod, []interface{}{pattern}, options.Preresolved{})
	if err != nil {
		return err
	}
	for {
		var gr naming.GlobReply
		if err := call.Recv(&gr); err != nil {
			break
		}
		if v, ok := gr.(naming.GlobReplyEntry); ok {
			fmt.Fprint(env.Stdout, v.Value.Name)
			for _, s := range v.Value.Servers {
				fmt.Fprintf(env.Stdout, " %s (Deadline %s)", s.Server, s.Deadline.Time)
			}
			fmt.Fprintln(env.Stdout)
		}
	}
	if err := call.Finish(); err != nil {
		return err
	}
	return nil
}

var cmdMount = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runMount),
	Name:     "mount",
	Short:    "Mounts a server <name> onto a mount table",
	Long:     "Mounts a server <name> onto a mount table",
	ArgsName: "<mount name> <name> <ttl> [L|M|R]",
	ArgsLong: `
<mount name> is a mount name on a mount table.

<name> is the rooted object name of the server.

<ttl> is the TTL of the new entry. It is a decimal number followed by a unit
suffix (s, m, h). A value of 0s represents an infinite duration.

[L|M|R] are mount options. L indicates that <name> is a leaf. M indicates that
<name> is a mounttable. R indicates that existing entries should be removed.
`,
}

func runMount(ctx *context.T, env *cmdline.Env, args []string) error {
	got := len(args)
	if got < 2 || got > 4 {
		return env.UsageErrorf("mount: incorrect number of arguments, expected 2, 3, or 4, got %d", got)
	}
	name := args[0]
	server := args[1]

	var flags naming.MountFlag
	var seconds uint32
	if got >= 3 {
		ttl, err := time.ParseDuration(args[2])
		if err != nil {
			return fmt.Errorf("TTL parse error: %v", err)
		}
		seconds = uint32(ttl.Seconds())
	}
	if got >= 4 {
		for _, c := range args[3] {
			switch c {
			case 'L':
				flags |= naming.Leaf
			case 'M':
				flags |= naming.MT
			case 'R':
				flags |= naming.Replace
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, name, "Mount", []interface{}{server, seconds, flags}, nil, options.Preresolved{}); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Name mounted successfully.")
	return nil
}

var cmdUnmount = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runUnmount),
	Name:     "unmount",
	Short:    "removes server <name> from the mount table",
	Long:     "removes server <name> from the mount table",
	ArgsName: "<mount name> <name>",
	ArgsLong: `
<mount name> is a mount name on a mount table.
<name> is the rooted object name of the server.
`,
}

func runUnmount(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("unmount: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, args[0], "Unmount", []interface{}{args[1]}, nil, options.Preresolved{}); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Unmount successful or name not mounted.")
	return nil
}

var cmdResolveStep = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runResolveStep),
	Name:     "resolvestep",
	Short:    "takes the next step in resolving a name.",
	Long:     "takes the next step in resolving a name.",
	ArgsName: "<mount name>",
	ArgsLong: `
<mount name> is a mount name on a mount table.
`,
}

func runResolveStep(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("mount: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	client := v23.GetClient(ctx)
	var entry naming.MountEntry
	if err := client.Call(ctx, args[0], "ResolveStep", nil, []interface{}{&entry}, options.Preresolved{}); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "Servers: %v Suffix: %q MT: %v\n", entry.Servers, entry.Name, entry.ServesMountTable)
	return nil
}

var cmdRoot = &cmdline.Command{
	Name:  "mounttable",
	Short: "sends commands to Vanadium mounttable services",
	Long: `
Command mounttable sends commands to Vanadium mounttable services.
`,
	Children: []*cmdline.Command{cmdGlob, cmdMount, cmdUnmount, cmdResolveStep},
}
