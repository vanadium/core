// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command deviced is used to launch, configure and manage the deviced daemon,
which implements the v.io/v23/services/device interfaces.

Usage:
   deviced [flags]
   deviced [flags] <command>

The deviced commands are:
   install     Install the device manager.
   uninstall   Uninstall the device manager.
   start       Start the device manager.
   stop        Stop the device manager.
   profile     Dumps profile for the device manager.
   help        Display help for commands or topics

The global flags are:
 -deviced-port=0
   the port number of assign to the device manager service. The hostname/IP
   address part of --v23.tcp.address is used along with this port. By default,
   the port is assigned by the OS.
 -name=
   name to publish the device manager at
 -neighborhood-name=
   if provided, it will enable sharing with the local neighborhood with the
   provided name. The address of the local mounttable will be published to the
   neighboorhood and everything in the neighborhood will be visible on the local
   mounttable.
 -restart-exit-code=0
   exit code to return when device manager should be restarted
 -use-pairing-token=false
   generate a pairing token for the device manager that will need to be provided
   when a device is claimed

 -agentsock=
   Path to the application's security agent socket.
 -alsologtostderr=true
   log to standard error as well as files
 -chown=false
   Change owner of files and directories given as command-line arguments to the
   user specified by this flag
 -dryrun=false
   Elides root-requiring systemcalls.
 -kill=false
   Kill process ids given as command-line arguments.
 -log_backtrace_at=:0
   when logging hits line file:N, emit a stack trace
 -log_dir=
   if non-empty, write log files to this directory
 -logdir=
   Path to the log directory.
 -logtostderr=false
   log to standard error instead of files
 -max_stack_buf_size=4292608
   max size in bytes of the buffer to use for logging stack traces
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -minuid=501
   UIDs cannot be less than this number.
 -progname=unnamed_app
   Visible name of the application, used in argv[0]
 -rm=false
   Remove the file trees given as command-line arguments.
 -run=
   Path to the application to exec.
 -stderrthreshold=2
   logs at or above this threshold go to stderr
 -time=false
   Dump timing information to stderr before exiting the program.
 -username=
   The UNIX user name used for the other functions of this tool.
 -v=0
   log level for V logs
 -v23.credentials=
   directory to use for storing security credentials
 -v23.i18n-catalogue=
   18n catalogue files to load, comma separated
 -v23.namespace.root=[/(dev.v.io:r:vprod:service:mounttabled)@ns.dev.v.io:8101]
   local namespace root; can be repeated to provided multiple roots
 -v23.permissions.file=map[]
   specify a perms file as <name>:<permsfile>
 -v23.permissions.literal=
   explicitly specify the runtime perms as a JSON-encoded access.Permissions.
   Overrides all --v23.permissions.file flags.
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
 -workspace=
   Path to the application's workspace directory.

Deviced install

Performs installation of device manager into V23_DEVICE_DIR (if the env var
set), or into the current dir otherwise

Usage:
   deviced install [flags] [-- <arguments for device manager>]

Arguments to be passed to the installed device manager

The deviced install flags are:
 -agent=
   path to security agent
 -devuser=
   if specified, device manager will run as this user. Provided by devicex but
   ignored .
 -from=
   if specified, performs the installation from the provided application
   envelope object name
 -init_helper=
   path to sysinit helper
 -init_mode=false
   if set, installs the device manager with the system init service manager
 -origin=
   if specified, self-updates will use this origin
 -restarter=
   path to restarter
 -session_mode=false
   if set, installs the device manager to run a single session. Otherwise, the
   device manager is configured to get restarted upon exit
 -single_user=false
   if set, performs the installation assuming a single-user system
 -suid_helper=
   path to suid helper

Deviced uninstall

Removes the device manager installation from V23_DEVICE_DIR (if the env var
set), or the current dir otherwise

Usage:
   deviced uninstall [flags]

The deviced uninstall flags are:
 -suid_helper=
   path to suid helper

Deviced start

Starts the device manager installed under from V23_DEVICE_DIR (if the env var
set), or the current dir otherwise

Usage:
   deviced start [flags]

Deviced stop

Stops the device manager installed under from V23_DEVICE_DIR (if the env var
set), or the current dir otherwise

Usage:
   deviced stop [flags]

Deviced profile

Prints the internal profile description for the device manager.

Usage:
   deviced profile [flags]

Deviced help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   deviced help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The deviced help flags are:
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
