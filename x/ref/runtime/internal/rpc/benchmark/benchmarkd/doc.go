// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command benchmarkd runs the benchmark server.

Usage:
   benchmarkd [flags]

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
 -test.bench=
   run only benchmarks matching `regexp`
 -test.benchmem=false
   print memory allocations for benchmarks
 -test.benchtime=1s
   run each benchmark for duration `d`
 -test.blockprofile=
   write a goroutine blocking profile to `file`
 -test.blockprofilerate=1
   set blocking profile `rate` (see runtime.SetBlockProfileRate)
 -test.count=1
   run tests and benchmarks `n` times
 -test.coverprofile=
   write a coverage profile to `file`
 -test.cpu=
   comma-separated `list` of cpu counts to run each test with
 -test.cpuprofile=
   write a cpu profile to `file`
 -test.failfast=false
   do not start new tests after the first test failure
 -test.list=
   list tests, examples, and benchmarks matching `regexp` then exit
 -test.memprofile=
   write an allocation profile to `file`
 -test.memprofilerate=0
   set memory allocation profiling `rate` (see runtime.MemProfileRate)
 -test.mutexprofile=
   write a mutex contention profile to the named file after execution
 -test.mutexprofilefraction=1
   if >= 0, calls runtime.SetMutexProfileFraction()
 -test.outputdir=
   write profiles to `dir`
 -test.parallel=<number of threads>
   run at most `n` tests in parallel
 -test.run=
   run only tests and examples matching `regexp`
 -test.short=false
   run smaller test suite to save time
 -test.testlogfile=
   write test action log to `file` (for use only by cmd/go)
 -test.timeout=0s
   panic test binary after duration `d` (default 0, timeout disabled)
 -test.trace=
   write an execution trace to `file`
 -test.v=false
   verbose: print additional output
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
