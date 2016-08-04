// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"os"

	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/flags"
)

// ParseFlags parses all registered flags taking into account overrides from other
// configuration and environment variables. It must be called by the profile and
// flags.RuntimeFlags() must be passed to the runtime initialization function. The
// profile can use or modify the flags as it pleases.
func ParseFlags(f *flags.Flags) error {
	config, err := exec.ReadConfigFromOSEnv()
	if err != nil {
		return err
	}
	// Parse runtime flags.
	var args map[string]string
	if config != nil {
		args = config.Dump()
	}
	return f.Parse(os.Args[1:], args)
}

// ParseFlagsAndConfigurGlobalLogger calls ParseFlags and then
// ConfigureGlobalLoggerFromFlags.
func ParseFlagsAndConfigureGlobalLogger(f *flags.Flags) error {
	if err := ParseFlags(f); err != nil {
		return err
	}
	if err := ConfigureGlobalLoggerFromFlags(); err != nil {
		return err
	}
	return nil
}

// ConfigureGlobalLoggerFromFlags configures the global logger from command
// line flags.  Should be called immediately after ParseFlags.
func ConfigureGlobalLoggerFromFlags() error {
	err := logger.Manager(logger.Global()).ConfigureFromFlags()
	if err != nil && !logger.IsAlreadyConfiguredError(err) {
		return err
	}
	return nil
}
