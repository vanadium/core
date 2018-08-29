// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command gclogs safely deletes old log files.

It looks for file names that match the format of files produced by the vlog
package, and deletes the ones that have not changed in the amount of time
specified by the --cutoff flag.

Only files produced by the same user as the one running the gclogs command are
considered for deletion.

Usage:
   gclogs [flags] <dir> ...

<dir> ... A list of directories where to look for log files.

The gclogs flags are:
 -cutoff=24h0m0s
   The age cut-off for a log file to be considered for garbage collection.
 -n=false
   If true, log files that would be deleted are shown on stdout, but not
   actually deleted.
 -program=.*
   A regular expression to apply to the program part of the log file name, e.g
   ".*test".
 -verbose=false
   If true, each deleted file is shown on stdout.

The global flags are:
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -time=false
   Dump timing information to stderr before exiting the program.
*/
package main
