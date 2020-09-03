// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command vrpc sends and receives Vanadium remote procedure calls.  It is used as
a generic client to interact with any Vanadium server.

Usage:
   vrpc [flags] <command>

The vrpc commands are:
   signature   Describe the interfaces of a Vanadium server
   call        Call a method of a Vanadium server
   identify    Reveal blessings presented by a Vanadium server
   help        Display help for commands or topics

The vrpc flags are:
 -json=false
   if true, output a JSON representation of the response
 -s=false
   if true, perform a shallow resolve

The global flags are:
 -v23.namespace.root=[/(dev.v.io:r:vprod:service:mounttabled)@ns.dev.v.io:8101]
   local namespace root; can be repeated to provided multiple roots

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
 -v23.permissions.file=
   specify a perms file as <name>:<permsfile>
 -v23.permissions.literal=
   explicitly specify the runtime perms as a JSON-encoded access.Permissions.
   Overrides all --v23.permissions.file flags
 -v23.proxy=
   object name of proxy service to use to export services across network
   boundaries
 -v23.proxy.limit=0
   max number of proxies to connect to when the policy is to connect to all
   proxies; 0 implies all proxies
 -v23.proxy.policy=
   policy for choosing from a set of available proxy instances
 -v23.tcp.address=
   address to listen on
 -v23.tcp.protocol=
   protocol to listen with
 -v23.vtrace.cache-size=1024
   The number of vtrace traces to store in memory
 -v23.vtrace.collect-regexp=
   Spans and annotations that match this regular expression will trigger trace
   collection
 -v23.vtrace.dump-on-shutdown=true
   If true, dump all stored traces on runtime shutdown
 -v23.vtrace.sample-rate=0
   Rate (from 0.0 to 1.0) to sample vtrace traces
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

Vrpc signature - Describe the interfaces of a Vanadium server

Signature connects to the Vanadium server identified by <server>.

If no [method] is provided, returns all interfaces implemented by the server.

If a [method] is provided, returns the signature of just that method.

Usage:
   vrpc signature [flags] <server> [method]

<server> identifies a Vanadium server.  It can either be the object address of
the server, or an object name that will be resolved to an end-point.

[method] is the optional server method name.

The vrpc signature flags are:
 -insecure=false
   If true, skip server authentication. This means that the client will reveal
   its blessings to servers that it may not recognize.
 -show-reserved=false
   if true, also show the signatures of reserved methods

 -json=false
   if true, output a JSON representation of the response
 -s=false
   if true, perform a shallow resolve

Vrpc call - Call a method of a Vanadium server

Call connects to the Vanadium server identified by <server> and calls the
<method> with the given positional [args...], returning results on stdout.

TODO(toddw): stdin is read for streaming arguments sent to the server.  An EOF
on stdin (e.g. via ^D) causes the send stream to be closed.

Regardless of whether the call is streaming, the main goroutine blocks for
streaming and positional results received from the server.

All input arguments (both positional and streaming) are specified as VDL
expressions, with commas separating multiple expressions.  Positional arguments
may also be specified as separate command-line arguments.  Streaming arguments
may also be specified as separate newline-terminated expressions.

The method signature is always retrieved from the server as a first step.  This
makes it easier to input complex typed arguments, since the top-level type for
each argument is implicit and doesn't need to be specified.

Usage:
   vrpc call [flags] <server> <method> [args...]

<server> identifies a Vanadium server.  It can either be the object address of
the server, or an object name that will be resolved to an end-point.

<method> is the server method to call.

[args...] are the positional input arguments, specified as VDL expressions.

The vrpc call flags are:
 -json=false
   if true, output a JSON representation of the response
 -s=false
   if true, perform a shallow resolve

Vrpc identify - Reveal blessings presented by a Vanadium server

Identify connects to the Vanadium server identified by <server> and dumps out
the blessings presented by that server (and the subset of those that are
considered valid by the principal running this tool) to standard output.

Usage:
   vrpc identify [flags] <server>

<server> identifies a Vanadium server.  It can either be the object address of
the server, or an object name that will be resolved to an end-point.

The vrpc identify flags are:
 -insecure=false
   If true, skip server authentication. This means that the client will reveal
   its blessings to servers that it may not recognize.

 -json=false
   if true, output a JSON representation of the response
 -s=false
   if true, perform a shallow resolve

Vrpc help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   vrpc help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The vrpc help flags are:
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
