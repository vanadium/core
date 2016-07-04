// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command binary manages the Vanadium binary repository.

Usage:
   binary [flags] <command>

The binary commands are:
   delete      Delete a binary
   download    Download a binary
   upload      Upload a binary or directory archive
   url         Fetch a download URL
   help        Display help for commands or topics

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
 -v23.credentials=
   directory to use for storing security credentials
 -v23.i18n-catalogue=
   18n catalogue files to load, comma separated
 -v23.namespace.root=[/(dev.v.io:r:vprod:service:mounttabled)@ns.dev.v.io:8101]
   local namespace root; can be repeated to provided multiple roots
 -v23.proxy=
   object name of proxy service to use to export services across network
   boundaries
 -v23.tcp.address=
   address to listen on
 -v23.tcp.protocol=wsh
   protocol to listen with
 -v23.vtrace.cache-size=1024
   The number of vtrace traces to store in memory.
 -v23.vtrace.collect-regexp=
   Spans and annotations that match this regular expression will trigger trace
   collection.
 -v23.vtrace.dump-on-shutdown=true
   If true, dump all stored traces on runtime shutdown.
 -v23.vtrace.sample-rate=0
   Rate (from 0.0 to 1.0) to sample vtrace traces.
 -v23.vtrace.v=0
   The verbosity level of the log messages to be captured in traces
 -vmodule=
   comma-separated list of globpattern=N settings for filename-filtered logging
   (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns baz or
   *az or b* but not by bar/baz or baz.go or az or b.*
 -vpath=
   comma-separated list of regexppattern=N settings for file pathname-filtered
   logging (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns
   foo/bar/baz or fo.*az or oo/ba or b.z but not by foo/bar/baz.go or fo*az

Binary delete - Delete a binary

Delete connects to the binary repository and deletes the specified binary

Usage:
   binary delete [flags] <von>

<von> is the vanadium object name of the binary to delete

Binary download - Download a binary

Download connects to the binary repository, downloads the specified binary, and
installs it to the specified location.

Usage:
   binary download [flags] <von> <location>

<von> is the vanadium object name of the binary to download <location> is the
path where the downloaded binary should be installed

The binary download flags are:
 -install=true
   Install the binary.  If false, it just downloads the binary and the media
   info file.

Binary upload - Upload a binary or directory archive

Upload connects to the binary repository and uploads the binary of the specified
file or archive of the specified directory. When successful, it writes the name
of the new binary to stdout.

Usage:
   binary upload [flags] <von> <filename>

<von> is the vanadium object name of the binary to upload <filename> is the name
of the file or directory to upload

Binary url - Fetch a download URL

Connect to the binary repository and fetch the download URL for the given
vanadium object name.

Usage:
   binary url [flags] <von>

<von> is the vanadium object name of the binary repository

Binary help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   binary help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The binary help flags are:
 -style=compact
   The formatting style for help output:
      compact   - Good for compact cmdline output.
      full      - Good for cmdline output, shows all global flags.
      godoc     - Good for godoc processing.
      shortonly - Only output short description.
   Override the default by setting the CMDLINE_STYLE environment variable.
 -width=<terminal width>
   Format output to this target width in runes, or unlimited if width < 0.
   Defaults to the terminal width if available.  Override the default by setting
   the CMDLINE_WIDTH environment variable.
*/
package main
