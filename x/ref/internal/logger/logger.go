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

type ManagedLogger interface {
	// Implement logging.Logging without referring to it.
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
	VDepth(depth int, level int) bool
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

	// Implement x/ref/internal.ManagedLog without referring to it.
	LogDir() string
	Stats() (Info, Error struct{ Lines, Bytes int64 })
	ConfigureFromFlags(opts ...vlog.LoggingOpts) error
	ConfigureFromArgs(opts ...vlog.LoggingOpts) error
	ExplicitlySetFlags() map[string]string
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
