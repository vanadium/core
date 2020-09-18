// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command vsh runs the Vanadium shell, a Tunnel client that can be used to run
shell commands or start an interactive shell on a remote tunneld server.

To open an interactive shell, use:
  vsh <object name>

To run a shell command, use:
  vsh <object name> <command to run>

The -L flag will forward connections from a local port to a remote address
through the tunneld service. The flag value is localAddress,remoteAddress. E.g.
  -L :14141,www.google.com:80

The -R flag will forward connections from a remote port on the tunneld service
to a local address. The flag value is remoteAddress,localAddress. E.g.
  -R :14141,www.google.com:80

vsh can't be used directly with tools like rsync because vanadium object names
don't look like traditional hostnames, which rsync doesn't understand. For
compatibility with such tools, vsh has a special feature that allows passing the
vanadium object name via the VSH_NAME environment variable.

  $ VSH_NAME=<object name> rsync -avh -e vsh /foo/* v23:/foo/

In this example, the "v23" host will be substituted with $VSH_NAME by vsh and
rsync will work as expected.

Usage:
   vsh [flags] <object name> [command]

<object name> is the Vanadium object name of the server to connect to.

[command] is the shell command and args to run, for non-interactive vsh.

The vsh flags are:
 -L=[]
   Forward local to remote, format is "localAddress,remoteAddress". May be
   repeated.
 -N=false
   Do not execute a shell.  Only do port forwarding.
 -R=[]
   Forward remote to local, format is "remoteAddress,localAddress". May be
   repeated.
 -T=false
   Disable pseudo-terminal allocation.
 -local-protocol=tcp
   Local network protocol for port forwarding.
 -remote-protocol=tcp
   Remote network protocol for port forwarding.
 -t=false
   Force allocation of pseudo-terminal.

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
*/
package main
