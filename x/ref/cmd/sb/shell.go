// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	isatty "github.com/mattn/go-isatty"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/cmd/sb/commands"
	"v.io/x/ref/cmd/sb/internal/reader"
)

var cmdSbShell = &cmdline.Command{
	Runner: commands.SbRunner(runSbShell),
	Name:   "sh",
	Short:  "Start a syncQL shell",
	Long: `
Connect to a database on the Syncbase service and start a syncQL shell.
`,
}

// Runs the shell.
// Takes commands as input, executes them, and prints out output.
func runSbShell(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	// Test if input is interactive and get reader.
	// TODO(ivanpi): This is hacky, it would be better for lib/cmdline to support IsTerminal.
	var input *reader.T
	stdinFile, ok := env.Stdin.(*os.File)
	isTerminal := ok && isatty.IsTerminal(stdinFile.Fd())
	if isTerminal {
		input = reader.NewInteractive()
	} else {
		input = reader.NewNonInteractive()
	}
	defer input.Close()

	// Read-exec loop.
	for true {
		// Read command.
		query, err := input.GetQueryWithTerminator('\n')
		if err != nil {
			if err == io.EOF && isTerminal {
				// ctrl-d
				fmt.Println()
			}
			break
		}

		// Exec command.
		fields := strings.Fields(query)
		if len(fields) > 0 {
			switch cmdName := fields[0]; cmdName {
			case "exit", "quit":
				return nil
			case "help":
				if err := help(fields[1:]); err != nil {
					return err
				}
			default:
				if err := runCommand(ctx, env, db, fields, isTerminal); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func runCommand(ctx *context.T, env *cmdline.Env, db syncbase.Database,
	fields []string, isTerminal bool) error {
	commands.SetCtx(ctx)
	commands.SetDB(db)
	resetFlags(cmdSb, false)
	if err := cmdline.ParseAndRun(cmdSb, env, fields); err != nil {
		if isTerminal {
			fmt.Fprintln(env.Stderr, "Error:", err)
		} else {
			// If running non-interactively, errors halt execution.
			return err
		}
	}
	return nil
}

func resetFlags(cmd *cmdline.Command, resetThisLevel bool) {
	if resetThisLevel {
		if cmd.ParsedFlags != nil {
			cmd.ParsedFlags.Visit(func(f *flag.Flag) {
				cmd.ParsedFlags.Set(f.Name, f.DefValue)
			})
		}
	}
	for _, nextCmd := range cmd.Children {
		resetFlags(nextCmd, true)
	}
}

func help(args []string) error {
	switch len(args) {
	case 0:
		fmt.Println("Commands:")
		for _, cmd := range commands.Commands {
			fmt.Printf("\t%s\t%s\n", cmd.Name, cmd.Short)
		}
		fmt.Println("\thelp\tPrint a list of all commands")
		fmt.Println("\texit\tEnd session (aliased to quit)")
		return nil

	case 1:
		cmdName := args[0]
		if cmdName == "help" {
			fmt.Println("Print a list of all commands, or useful information about a single command.")
			fmt.Println()
			fmt.Println("Usage:")
			fmt.Println("\thelp [command_name]")
		} else {
			cmd, err := commands.GetCommand(cmdName)
			if err != nil {
				return err
			}
			commands.PrintUsage(cmd)
		}
		return nil

	default:
		return fmt.Errorf("too many arguments")
	}
}
