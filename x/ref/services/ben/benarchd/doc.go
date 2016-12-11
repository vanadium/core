// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command benarchd implements a benchmark archival daemon.

It provides a Vanadium RPC service to accept results to archive (typically
provided by the benup tool - https://godoc.org/v.io/x/ref/services/ben/benup)
and a web-interface to browse the results.

A SQL database is used for persistent storage, configured via the --store flag.

Usage:
   benarchd [flags]

The benarchd flags are:
 -assets=
   If set, the directory containing assets (template definitions, css,
   javascript files etc.) to use in the web interface. If not set, compiled-in
   assets will be used instead.
 -exthttp=
   The address of the HTTP server to advertise externally, typically used if the
   HTTP server is running behind a proxy or on a machine that is unaware of its
   publicly accessible hostname/IP address. If empty, derived from --http.
 -http=127.0.0.1:0
   Address on which to serve HTTP requests
 -name=
   Vanadium object name to export this service under
 -store=
   Specification of the persistent store to use. Format: <engine>:<parameters>,
   where <engine> can be 'sqlite3', 'mysql' or 'sqlconfig'. For 'sqlconfig',
   <parameters> is a path to a file containing a JSON-serialized SqlConfig
   (https://godoc.org/v.io/x/lib/dbutil#SqlConfig), for the others it is the
   data source name (e.g., filename for sqlite3)

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
*/
package main
