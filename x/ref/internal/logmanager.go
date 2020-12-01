// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"v.io/v23/logging"
	"v.io/x/lib/vlog"
)

// ManageLog defines the methods for managing and configuring a logger.
type ManageLog interface {

	// LogDir returns the directory where the log files are written.
	LogDir() string

	// Stats returns stats on how many lines/bytes haven been written to
	// this set of logs.
	Stats() (Info, Error struct{ Lines, Bytes int64 })

	// ConfigureLoggerFromFlags will configure the supplied logger using
	// command line flags.
	ConfigureFromFlags(opts ...vlog.LoggingOpts) error

	// ConfigureFromArgs will configure the supplied logger using the supplied
	// arguments.
	ConfigureFromArgs(opts ...vlog.LoggingOpts) error

	// ExplicitlySetFlags returns a map of the logging command line flags and their
	// values formatted as strings.  Only the flags that were explicitly set are
	// returned. This is intended for use when an application needs to know what
	// value the flags were set to, for example when creating subprocesses.
	ExplicitlySetFlags() map[string]string
}

// ManagedLogger consists of both the core logging operations and
// management operations. All replacements for the module
// v.io/x/ref/internal/logger must implement this interface.
type ManagedLogger interface {
	logging.Logger
	ManageLog
}
