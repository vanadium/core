// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command benchmark runs the benchmark client.

Usage:
   benchmark [flags]

The benchmark flags are:
 -chunk_count=0
   Number of chunks to send per streaming RPC (if zero, use non-streaming RPC).
 -iterations=100
   Number of iterations to run.
 -mux_chunk_count=0
   Number of chunks to send in background.
 -mux_payload_size=0
   Size of payload to send in background.
 -payload_size=0
   Size of payload in bytes.
 -server=
   Address of the server to connect to.

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
   regular expression to select benchmarks to run
 -test.benchmem=false
   print memory allocations for benchmarks
 -test.benchtime=1s
   approximate run time for each benchmark
 -test.blockprofile=
   write a goroutine blocking profile to the named file after execution
 -test.blockprofilerate=1
   if >= 0, calls runtime.SetBlockProfileRate()
 -test.count=1
   run tests and benchmarks `n` times
 -test.coverprofile=
   write a coverage profile to the named file after execution
 -test.cpu=
   comma-separated list of number of CPUs to use for each test
 -test.cpuprofile=
   write a cpu profile to the named file during execution
 -test.memprofile=
   write a memory profile to the named file after execution
 -test.memprofilerate=0
   if >=0, sets runtime.MemProfileRate
 -test.outputdir=
   directory in which to write profiles
 -test.parallel=<number of threads>
   maximum test parallelism
 -test.run=
   regular expression to select tests and examples to run
 -test.short=false
   run smaller test suite to save time
 -test.timeout=0
   if positive, sets an aggregate time limit for all tests
 -test.trace=
   write an execution trace to the named file after execution
 -test.v=false
   verbose: print additional output
 -time=false
   Dump timing information to stderr before exiting the program.
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
*/
package main
