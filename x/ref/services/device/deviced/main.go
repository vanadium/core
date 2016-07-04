// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go .

package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
)

func main() {
	// TODO(caprita): Remove this once we have a way to set the GOMAXPROCS
	// environment variable persistently for device manager.
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	// Make deviced the leader of a new process group.
	if err := syscall.Setpgid(0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "Setpgid failed: %v\n", err)
	}

	rootCmd := &cmdline.Command{
		Name:  "deviced",
		Short: "launch, configure and manage the deviced daemon",
		Long: `
Command deviced is used to launch, configure and manage the deviced daemon,
which implements the v.io/v23/services/device interfaces.
`,
		Children: []*cmdline.Command{cmdInstall, cmdUninstall, cmdStart, cmdStop, cmdProfile},
		Runner:   v23cmd.RunnerFunc(runServer),
	}
	cmdline.Main(rootCmd)
}
