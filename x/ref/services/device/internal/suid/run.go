// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suid

import (
	"flag"
)

func Run(environ []string) error {
	var work WorkParameters
	if err := work.ProcessArguments(flag.CommandLine, environ); err != nil {
		return err
	}

	if work.remove {
		return work.Remove()
	}

	if work.kill {
		return work.Kill()
	}

	if err := work.Chown(); err != nil {
		return err
	}

	if work.chown {
		// We were called with --chown, and Chown() was called above.
		// There is nothing else to do.
		return nil
	}

	return work.Exec()
}
