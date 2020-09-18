// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command mounttable sends commands to Vanadium mounttable services.

Usage:
   mounttable [flags] <command>

The mounttable commands are:
   glob        returns all matching entries in the mount table
   mount       Mounts a server <name> onto a mount table
   unmount     removes server <name> from the mount table
   resolvestep takes the next step in resolving a name.
   help        Display help for commands or topics

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

Mounttable glob - returns all matching entries in the mount table

returns all matching entries in the mount table

Usage:
   mounttable glob [flags] [<mount name>] <pattern>

<mount name> is a mount name on a mount table.  Defaults to namespace root.
<pattern> is a glob pattern that is matched against all the entries below the
specified mount name.

Mounttable mount

Mounts a server <name> onto a mount table

Usage:
   mounttable mount [flags] <mount name> <name> <ttl> [L|M|R]

<mount name> is a mount name on a mount table.

<name> is the rooted object name of the server.

<ttl> is the TTL of the new entry. It is a decimal number followed by a unit
suffix (s, m, h). A value of 0s represents an infinite duration.

[L|M|R] are mount options. L indicates that <name> is a leaf. M indicates that
<name> is a mounttable. R indicates that existing entries should be removed.

Mounttable unmount

removes server <name> from the mount table

Usage:
   mounttable unmount [flags] <mount name> <name>

<mount name> is a mount name on a mount table. <name> is the rooted object name
of the server.

Mounttable resolvestep

takes the next step in resolving a name.

Usage:
   mounttable resolvestep [flags] <mount name>

<mount name> is a mount name on a mount table.

Mounttable help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   mounttable help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The mounttable help flags are:
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
