// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command restarter runs a child command and optionally restarts it depending on
the setting of the --restart-exit-code flag.

Example:
 # Prints "foo" just once.
 $ restarter echo foo
 # Prints "foo" in a loop.
 $ restarter --restart-exit-code=13 bash -c "echo foo; sleep 1; exit 13"
 # Prints "foo" just once.
 $ restarter --restart-exit-code=\!13 bash -c "echo foo; sleep 1; exit 13"

Usage:
   restarter [flags] command [command_args...]

The command is started as a subprocess with the given [command_args...].

The restarter flags are:
 -restart-exit-code=
   If non-empty, will restart the command when it exits, provided that the
   command's exit code matches the value of this flag.  The value must be an
   integer, or an integer preceded by '!' (in which case all exit codes except
   the flag will trigger a restart).

The global flags are:
 -alsologtostderr=true
   log to standard error as well as files
 -log_backtrace_at=:0
   when logging hits line file:N, emit a stack trace
 -log_dir=
   if non-empty, write log files to this directory
 -logtostderr=false
   log to standard error instead of files
 -max_stack_buf_size=4292608
   max size in bytes of the buffer to use for logging stack traces
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -stderrthreshold=2
   logs at or above this threshold go to stderr
 -time=false
   Dump timing information to stderr before exiting the program.
 -v=0
   log level for V logs
 -vmodule=
   comma-separated list of globpattern=N settings for filename-filtered logging
   (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns baz or
   *az or b* but not by bar/baz or baz.go or az or b.*
 -vpath=
   comma-separated list of regexppattern=N settings for file pathname-filtered
   logging (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns
   foo/bar/baz or fo.*az or oo/ba or b.z but not by foo/bar/baz.go or fo*az
*/
package main
