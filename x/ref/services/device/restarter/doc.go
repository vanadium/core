// Copyright 2015 The Vanadium Authors. All rights reserved.
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
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -time=false
   Dump timing information to stderr before exiting the program.
*/
package main
