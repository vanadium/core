// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command inithelper manages services for a variety of platforms and init
// systems, such as upstart, systemd etc.
package main

// TODO(caprita): The separation of responsibilities between root/non-root code
// can be shifted away from root a bit more, by having the init daemon files
// created as non-root and root just installs files and runs a specific set of
// commands.  Figure out if worth it.  Also consider combining with suidhelper.
// For now they're separate since we don't always need both at the same time.

// TODO(caprita): Add unit test.

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"v.io/x/ref/services/device/internal/sysinit"
)

func usage() {
	const usage = `Usage:
%s [flags] [command]

 Flags:
%s
 Command:
  print: prints the file that would be installed
  install: installs the service
  uninstall: uninstalls the service
  start: starts the service
  stop: stops the service
`
	var flagDefaults bytes.Buffer
	flag.CommandLine.SetOutput(&flagDefaults)
	flag.CommandLine.PrintDefaults()
	flag.CommandLine.SetOutput(nil)
	fmt.Fprintf(os.Stderr, usage, os.Args[0], flagDefaults.String())
}

func main() {
	fmt.Fprintln(os.Stderr, os.Args)
	if os.Geteuid() != 0 && os.Getuid() != 0 {
		fmt.Fprintln(os.Stderr, "uid is ", os.Getuid(), ", effective uid is ", os.Geteuid())
		fmt.Fprintln(os.Stderr, "inithelper is not root. Is your filesystem mounted with nosuid?")
		os.Exit(1)
	}

	flag.Usage = usage

	// nolint: staticcheck //lint:ignore SA4006 - this is a real false positive.
	sdFlag := flag.String("service_description", "", "File containing a JSON-encoded sysinit.ServiceDescription object.")
	systemFlag := flag.String("system", sysinit.InitSystem(), "System label, to select the appropriate sysinit mechanism.")
	flag.Parse()
	if *sdFlag == "" {
		fmt.Fprintf(os.Stderr, "--service_description must be set.\n")
		flag.Usage()
		os.Exit(1)
	}
	if *systemFlag == "" {
		fmt.Fprintf(os.Stderr, "--system must be set.\n")
		flag.Usage()
		os.Exit(1)
	}
	var sd sysinit.ServiceDescription
	if err := sd.LoadFrom(*sdFlag); err != nil {
		fmt.Fprintf(os.Stderr, "LoadFrom(%v) failed: %v\n", *sdFlag, err)
		os.Exit(2)
	}
	si := sysinit.New(*systemFlag, &sd)
	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Command must be specified.\n")
		flag.Usage()
		os.Exit(1)
	}
	switch args[0] {
	case "print":
		if err := si.Print(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to print %v: %s\n", si, err)
			os.Exit(2)
		}
	case "install":
		if err := si.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install %v: %s\n", si, err)
			os.Exit(2)
		}
	case "uninstall":
		if err := si.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to uninstall %v: %s\n", si, err)
			os.Exit(2)
		}
	case "start":
		if err := si.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start %v: %s\n", si, err)
			os.Exit(2)
		}
	case "stop":
		if err := si.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop %v: %s\n", si, err)
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "Invalid command: %s\n", args[0])
		flag.Usage()
		os.Exit(1)
	}
}
