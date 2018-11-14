// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"os"

	"v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/flags"
)

// ParseFlagsIncV23Env parses all registered flags taking into account overrides
// from other configuration and environment variables such as V23_EXEC_CONFIG
// It must be called by the factory and flags.RuntimeFlags() must be passed to
// the runtime initialization function.
func ParseFlagsIncV23Env(f *flags.Flags) error {
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
