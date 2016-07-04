// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"flag"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"

	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	vsignals "v.io/x/ref/lib/signals"
)

const pkgPath = "v.io/x/ref/services/device/restarter"

var (
	errCantParseRestartExitCode = verror.Register(pkgPath+".errCantParseRestartExitCode", verror.NoRetry, "{1:}{2:} Failed to parse restart exit code{:_}")

	restartExitCode string
)

func main() {
	cmdRestarter.Flags.StringVar(&restartExitCode, "restart-exit-code", "", "If non-empty, will restart the command when it exits, provided that the command's exit code matches the value of this flag.  The value must be an integer, or an integer preceded by '!' (in which case all exit codes except the flag will trigger a restart).")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRestarter)
}

var cmdRestarter = &cmdline.Command{
	Runner: cmdline.RunnerFunc(runRestarter),
	Name:   "restarter",
	Short:  "Runs a child command and restarts it depending on its exit code",
	Long: `
Command restarter runs a child command and optionally restarts it depending on the setting of the --restart-exit-code flag.

Example:
 # Prints "foo" just once.
 $ restarter echo foo
 # Prints "foo" in a loop.
 $ restarter --restart-exit-code=13 bash -c "echo foo; sleep 1; exit 13"
 # Prints "foo" just once.
 $ restarter --restart-exit-code=\!13 bash -c "echo foo; sleep 1; exit 13"
`,
	ArgsName: "command [command_args...]",
	ArgsLong: `
The command is started as a subprocess with the given [command_args...].
`,
}

func runRestarter(env *cmdline.Env, args []string) error {
	var restartOpts restartOptions
	if err := restartOpts.parse(); err != nil {
		return env.UsageErrorf("%v", err)
	}

	if len(args) == 0 {
		return env.UsageErrorf("command not specified")
	}

	exitCode := 0
	for {
		// Run the client and wait for it to finish.
		cmd := exec.Command(flag.Args()[0], flag.Args()[1:]...)
		cmd.Stdin = env.Stdin
		cmd.Stdout = env.Stdout
		cmd.Stderr = env.Stderr

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("Error starting child: %v", err)
		}
		shutdown := make(chan struct{})
		go func() {
			// TODO(caprita): Revisit why we're only sending down
			// the first signal we get to the child (instead of
			// sending all signals we can handle).
			select {
			case sig := <-vsignals.ShutdownOnSignals(nil):
				// TODO(caprita): Should we also relay double
				// signal to the child?  That currently just
				// force exits the current process.
				if sig == vsignals.STOP {
					sig = syscall.SIGTERM
				}
				cmd.Process.Signal(sig)
			case <-shutdown:
			}
		}()
		cmd.Wait()
		close(shutdown)
		exitCode = cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
		if !restartOpts.restart(exitCode) {
			break
		}
	}
	if exitCode != 0 {
		return cmdline.ErrExitCode(exitCode)
	}
	return nil
}

type restartOptions struct {
	enabled, unless bool
	code            int
}

func (opts *restartOptions) parse() error {
	code := restartExitCode
	if code == "" {
		return nil
	}
	opts.enabled = true
	if code[0] == '!' {
		opts.unless = true
		code = code[1:]
	}
	var err error
	if opts.code, err = strconv.Atoi(code); err != nil {
		return verror.New(errCantParseRestartExitCode, nil, err)
	}
	return nil
}

func (opts *restartOptions) restart(exitCode int) bool {
	return opts.enabled && opts.unless != (exitCode == opts.code)
}
