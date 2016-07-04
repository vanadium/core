// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/services/application"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
)

type configFlag device.Config

func (c *configFlag) String() string {
	jsonConfig, _ := json.Marshal(c)
	return string(jsonConfig)
}
func (c *configFlag) Set(s string) error {
	if err := json.Unmarshal([]byte(s), c); err != nil {
		return fmt.Errorf("Unmarshal(%v) failed: %v", s, err)
	}
	return nil
}

var configOverride configFlag = configFlag{}

type packagesFlag application.Packages

func (c *packagesFlag) String() string {
	jsonPackages, _ := json.Marshal(c)
	return string(jsonPackages)
}
func (c *packagesFlag) Set(s string) error {
	if err := json.Unmarshal([]byte(s), c); err != nil {
		return fmt.Errorf("Unmarshal(%v) failed: %v", s, err)
	}
	return nil
}

var packagesOverride packagesFlag = packagesFlag{}

var cmdInstall = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runInstall),
	Name:     "install",
	Short:    "Install the given application.",
	Long:     "Install the given application and print the name of the new installation.",
	ArgsName: "<device> <application>",
	ArgsLong: `
<device> is the vanadium object name of the device manager's app service.

<application> is the vanadium object name of the application.
`,
}

func init() {
	cmdInstall.Flags.Var(&configOverride, "config", "JSON-encoded device.Config object, of the form: '{\"flag1\":\"value1\",\"flag2\":\"value2\"}'")
	cmdInstall.Flags.Var(&packagesOverride, "packages", "JSON-encoded application.Packages object, of the form: '{\"pkg1\":{\"File\":\"object name 1\"},\"pkg2\":{\"File\":\"object name 2\"}}'")
}

func runInstall(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("install: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	deviceName, appName := args[0], args[1]
	appID, err := device.ApplicationClient(deviceName).Install(ctx, appName, device.Config(configOverride), application.Packages(packagesOverride))
	// Reset the value for any future invocations of "install" or
	// "install-local" (we run more than one command per process in unit
	// tests).
	configOverride = configFlag{}
	packagesOverride = packagesFlag{}
	if err != nil {
		return fmt.Errorf("Install failed: %v", err)
	}
	fmt.Fprintf(env.Stdout, "%s\n", naming.Join(deviceName, appID))
	return nil
}
