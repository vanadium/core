// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go .

package main

import (
	"fmt"
	"time"

	"v.io/v23/context"
	"v.io/v23/services/build"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/profile"
	"v.io/x/ref/services/repository"
)

func main() {
	cmdline.Main(cmdRoot)
}

func init() {
	cmdline.HideGlobalFlagsExcept()
}

var cmdLabel = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runLabel),
	Name:     "label",
	Short:    "Shows a human-readable profile key for the profile.",
	Long:     "Shows a human-readable profile key for the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runLabel(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("label: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p := repository.ProfileClient(name)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	label, err := p.Label(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, label)
	return nil
}

var cmdDescription = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runDescription),
	Name:     "description",
	Short:    "Shows a human-readable profile description for the profile.",
	Long:     "Shows a human-readable profile description for the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runDescription(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("description: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p := repository.ProfileClient(name)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	desc, err := p.Description(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, desc)
	return nil
}

var cmdSpecification = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runSpecification),
	Name:     "specification",
	Short:    "Shows the specification of the profile.",
	Long:     "Shows the specification of the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runSpecification(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("specification: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p := repository.ProfileClient(name)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	spec, err := p.Specification(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "%#v\n", spec)
	return nil
}

var cmdPut = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runPut),
	Name:     "put",
	Short:    "Sets a placeholder specification for the profile.",
	Long:     "Sets a placeholder specification for the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runPut(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("put: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p := repository.ProfileClient(name)

	// TODO(rthellend): Read an actual specification from a file.
	spec := profile.Specification{
		Arch:        build.ArchitectureAmd64,
		Description: "Example profile to test the profile manager implementation.",
		Format:      build.FormatElf,
		Libraries:   map[profile.Library]struct{}{profile.Library{Name: "foo", MajorVersion: "1", MinorVersion: "0"}: struct{}{}},
		Label:       "example",
		Os:          build.OperatingSystemLinux,
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := p.Put(ctx, spec); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Profile added successfully.")
	return nil
}

var cmdRemove = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runRemove),
	Name:     "remove",
	Short:    "removes the profile specification for the profile.",
	Long:     "removes the profile specification for the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runRemove(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("remove: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p := repository.ProfileClient(name)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := p.Remove(ctx); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Profile removed successfully.")
	return nil
}

var cmdRoot = &cmdline.Command{
	Name:  "profile",
	Short: "manages the Vanadium profile repository",
	Long: `
Command profile manages the Vanadium profile repository.
`,
	Children: []*cmdline.Command{cmdLabel, cmdDescription, cmdSpecification, cmdPut, cmdRemove},
}
