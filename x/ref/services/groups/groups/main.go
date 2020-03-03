// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"v.io/v23/context"
	"v.io/v23/security/access"
	"v.io/v23/services/groups"
	"v.io/x/lib/cmdline"
	"v.io/x/lib/set"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
)

type relateResult struct {
	Remainder      map[string]struct{}
	Approximations []groups.Approximation
	Version        string
}

var (
	flagPermFile      string
	flagVersion       string
	flagApproximation string

	cmdCreate = &cmdline.Command{
		Name:     "create",
		Short:    "Creates a blessing pattern group",
		Long:     "Creates a blessing pattern group.",
		ArgsName: "<von> <patterns...>",
		ArgsLong: `
<von> is the vanadium object name of the group to create

<patterns...> is a list of blessing pattern chunks
`,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			// Process command-line arguments.
			if want, got := 1, len(args); want > got {
				return env.UsageErrorf("create: unexpected number of arguments, want at least %d, got %d", want, got)
			}
			von := args[0]
			var patterns []groups.BlessingPatternChunk
			for _, pattern := range args[1:] {
				patterns = append(patterns, groups.BlessingPatternChunk(pattern))
			}
			// Process command-line flags.
			var permissions access.Permissions
			if flagPermFile != "" {
				file, err := os.Open(flagPermFile)
				if err != nil {
					return fmt.Errorf("Open(%v) failed: %v", flagPermFile, err)
				}
				permissions, err = access.ReadPermissions(file)
				if err != nil {
					return err
				}
			}
			// Invoke the "create" RPC.
			client := groups.GroupClient(von)
			return client.Create(ctx, permissions, patterns)
		}),
	}

	cmdDelete = &cmdline.Command{
		Name:     "delete",
		Short:    "Delete a blessing group",
		Long:     "Delete a blessing group.",
		ArgsName: "<von>",
		ArgsLong: "<von> is the vanadium object name of the group",
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			// Process command-line arguments.
			if want, got := 1, len(args); want != got {
				return env.UsageErrorf("delete: unexpected number of arguments, want %d, got %d", want, got)
			}
			von := args[0]
			// Invoke the "delete" RPC.
			client := groups.GroupClient(von)
			return client.Delete(ctx, flagVersion)
		}),
	}

	cmdAdd = &cmdline.Command{
		Name:     "add",
		Short:    "Adds a blessing pattern to a group",
		Long:     "Adds a blessing pattern to a group.",
		ArgsName: "<von> <pattern>",
		ArgsLong: `
<von> is the vanadium object name of the group

<pattern> is the blessing pattern chunk to add
`,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			// Process command-line arguments.
			if want, got := 2, len(args); want != got {
				return env.UsageErrorf("add: unexpected number of arguments, want %d, got %d", want, got)
			}
			von, pattern := args[0], args[1]
			// Invoke the "add" RPC.
			client := groups.GroupClient(von)
			return client.Add(ctx, groups.BlessingPatternChunk(pattern), flagVersion)
		}),
	}

	cmdRemove = &cmdline.Command{
		Name:     "remove",
		Short:    "Removes a blessing pattern from a group",
		Long:     "Removes a blessing pattern from a group.",
		ArgsName: "<von> <pattern>",
		ArgsLong: `
<von> is the vanadium object name of the group

<pattern> is the blessing pattern chunk to add
`,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			// Process command-line arguments.
			if want, got := 2, len(args); want != got {
				return env.UsageErrorf("remove: unexpected number of arguments, want %d, got %d", want, got)
			}
			von, pattern := args[0], args[1]
			// Invoke the "remove" RPC.
			client := groups.GroupClient(von)
			return client.Remove(ctx, groups.BlessingPatternChunk(pattern), flagVersion)
		}),
	}

	cmdRelate = &cmdline.Command{
		Name:  "relate",
		Short: "Relate a set of blessing to a group",
		Long: `
Relate a set of blessing to a group. The result is returned as a
JSON-encoded output.

NOTE: This command exists primarily for debugging purposes. In
particular, invocations of the Relate RPC are expected to be mainly
issued by the authorization logic.
`,
		ArgsName: "<von> <blessings>",
		ArgsLong: `
<von> is the vanadium object name of the group

<blessings...> is a list of blessings
`,
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			// Process command-line arguments.
			if want, got := 1, len(args); want > got {
				return env.UsageErrorf("relate: unexpected number of arguments, want at least %d, got %d", want, got)
			}
			von := args[0]
			blessings := set.String.FromSlice(args[1:])
			// Process command-line flags.
			var hint groups.ApproximationType
			switch flagApproximation {
			case "under":
				hint = groups.ApproximationTypeUnder
			case "over":
				hint = groups.ApproximationTypeOver
			}
			// Invoke the "relate" RPC.
			client := groups.GroupClient(von)
			remainder, approximations, version, err := client.Relate(ctx, blessings, hint, flagVersion, nil)
			if err != nil {
				return err
			}
			result := relateResult{
				Remainder:      remainder,
				Approximations: approximations,
				Version:        version,
			}
			bytes, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("MarshalIndent(%v) failed: %v", result, err)
			}
			fmt.Fprintf(env.Stdout, "%v\n", string(bytes))
			return nil
		}),
	}

	cmdGet = &cmdline.Command{
		Name:     "get",
		Short:    "Returns entries of a group",
		Long:     "Returns entries of a group.",
		ArgsName: "<von>",
		ArgsLong: "<von> is the vanadium object name of the group",
		Runner: v23cmd.RunnerFunc(func(ctx *context.T, env *cmdline.Env, args []string) error {
			// TODO(jsimsa): Implement once the corresponding server-side
			// API is designed.
			return fmt.Errorf(`the "groups get ..." sub-command is not implemented`)
		}),
	}

	cmdRoot = &cmdline.Command{
		Name:     "groups",
		Short:    "creates and manages Vanadium groups of blessing patterns",
		Long:     "Command groups creates and manages Vanadium groups of blessing patterns.",
		Children: []*cmdline.Command{cmdCreate, cmdDelete, cmdAdd, cmdRemove, cmdRelate, cmdGet},
	}
)

func init() {
	cmdCreate.Flags.StringVar(&flagPermFile, "permissions", "", "Path to a permissions file")
	cmdDelete.Flags.StringVar(&flagVersion, "version", "", "Identifies group version")
	cmdAdd.Flags.StringVar(&flagVersion, "version", "", "Identifies group version")
	cmdRemove.Flags.StringVar(&flagVersion, "version", "", "Identifies group version")
	cmdRelate.Flags.StringVar(&flagVersion, "version", "", "Identifies group version")
	cmdRelate.Flags.StringVar(&flagApproximation, "approximation", "under",
		"Identifies the type of approximation to use; supported values = (under, over)",
	)
}

func main() {
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}
