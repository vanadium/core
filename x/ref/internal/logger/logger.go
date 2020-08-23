// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package logger provides access to an implementation of v23/logger.Logging
// for use within a runtime implementation. There is pre-created "global" logger
// and the ability to create new loggers.
package logger

import (
	"v.io/x/lib/vlog"

	"v.io/v23/context"
	"v.io/v23/logging"
)

// Global returns the global logger.
func Global() logging.Logger {
	return vlog.Log
}

// NewLogger creates a new logger with the supplied name.
func NewLogger(name string) logging.Logger {
	return vlog.NewLogger(name)
}

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

type dummy struct{}

func (*dummy) LogDir() string { return "" }
func (*dummy) Stats() (infoStats, errorStats struct{ Lines, Bytes int64 }) {
	return struct{ Lines, Bytes int64 }{0, 0}, struct{ Lines, Bytes int64 }{0, 0}
}
func (*dummy) ConfigureFromFlags(opts ...vlog.LoggingOpts) error { return nil }
func (*dummy) ConfigureFromArgs(opts ...vlog.LoggingOpts) error  { return nil }

func (*dummy) ExplicitlySetFlags() map[string]string { return nil }

// Manager determines if the supplied logging.Logger implements ManageLog and if so
// returns an instance of it. If it doesn't implement ManageLog then Manager
// will return a dummy implementation that is essentially a no-op. It is
// always safe to use it as: logger.Manager(logger.Global()).LogDir() for example.
func Manager(logger logging.Logger) ManageLog {
	if vl, ok := logger.(ManageLog); ok {
		return vl
	}
	// If the logger is a context.T then ask it for its implementation
	// of logging.Logger and look for ManageLog being implemented by it,
	// since context.T can never implement ManageLog itself.
	if ctx, ok := logger.(*context.T); ok {
		if l := context.LoggerFromContext(ctx); l != nil {
			return Manager(l)
		}
	}
	return &dummy{}
}

// IsAlreadyConfiguredError returns true if the err parameter indicates
// the the logger has already been configured.
func IsAlreadyConfiguredError(err error) bool {
	return err == vlog.ErrConfigured
}
