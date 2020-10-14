// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package logger provides access to an implementation of v23/logger.Logging
// for use within a runtime implementation. There is pre-created "global" logger
// and the ability to create new loggers.
package logger

import (
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

type ManagedLogger interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	InfoDepth(depth int, args ...interface{})
	InfoStack(all bool)
	Error(args ...interface{})
	ErrorDepth(depth int, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	FatalDepth(depth int, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panic(args ...interface{})
	PanicDepth(depth int, args ...interface{})
	Panicf(format string, args ...interface{})
	V(level int) bool
	VI(level int) interface {
		Info(args ...interface{})
		Infof(format string, args ...interface{})
		InfoDepth(depth int, args ...interface{})
		InfoStack(all bool)
	}
	VIDepth(depth int, level int) interface {
		Info(args ...interface{})
		Infof(format string, args ...interface{})
		InfoDepth(depth int, args ...interface{})
		InfoStack(all bool)
	}
	FlushLog()
	ManageLog
}

// Global returns the global logger.
func Global() ManagedLogger {
	return vlog.Log
}

// NewLogger creates a new logger with the supplied name.
func NewLogger(name string) ManagedLogger {
	return vlog.NewLogger(name)
}

// IsAlreadyConfiguredError returns true if the err parameter indicates
// the the logger has already been configured.
func IsAlreadyConfiguredError(err error) bool {
	return err == vlog.ErrConfigured
}
