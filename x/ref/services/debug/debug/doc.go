// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command debug supports debugging Vanadium servers.

Usage:
   debug [flags] <command>

The debug commands are:
   glob        Returns all matching entries from the namespace.
   vtrace      Returns vtrace traces.
   logs        Accesses log files
   stats       Accesses stats
   pprof       Accesses profiling data
   browse      Starts an interactive interface for debugging
   delegate    Create credentials to delegate debugging to another user
   discovery   Scan and advertise using the v23 discovery API
   help        Display help for commands or topics

The debug flags are:
 -timeout=10s
   Time to wait for various RPCs

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
 -v23.virtualized.advertise-private-addresses=
   if set the process will also advertise its private addresses
 -v23.virtualized.disallow-native-fallback=false
   if set, a failure to detect the requested virtualization provider will result
   in an error, otherwise, native mode is used
 -v23.virtualized.dns.public-name=
   if set the process will use the supplied dns name (and port) without
   resolution for its entry in the mounttable
 -v23.virtualized.docker=
   set if the process is running in a docker container and needs to configure
   itself differently therein
 -v23.virtualized.provider=
   the name of the virtualization/cloud provider hosting this process if the
   process needs to configure itself differently therein
 -v23.virtualized.tcp.public-address=
   if set the process will use this address (resolving via dns if appropriate)
   for its entry in the mounttable
 -v23.virtualized.tcp.public-protocol=
   if set the process will use this protocol for its entry in the mounttable
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

Debug glob

Returns all matching entries from the namespace.

Usage:
   debug glob [flags] <pattern> ...

<pattern> is a glob pattern to match.

The debug glob flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug vtrace

Returns matching vtrace traces (or all stored traces if no ids are given).

Usage:
   debug vtrace [flags] <name> [id ...]

<name> is the name of a vtrace object. [id] is a vtrace trace id.

The debug vtrace flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug logs - Accesses log files

Accesses log files

Usage:
   debug logs [flags] <command>

The debug logs commands are:
   read        Reads the content of a log file object.
   size        Returns the size of a log file object.

The debug logs flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug logs read

Reads the content of a log file object.

Usage:
   debug logs read [flags] <name>

<name> is the name of the log file object.

The debug logs read flags are:
 -f=false
   When true, read will wait for new log entries when it reaches the end of the
   file.
 -n=-1
   The number of log entries to read.
 -o=0
   The position, in bytes, from which to start reading the log file.
 -v=false
   When true, read will be more verbose.

 -timeout=10s
   Time to wait for various RPCs

Debug logs size

Returns the size of a log file object.

Usage:
   debug logs size [flags] <name>

<name> is the name of the log file object.

The debug logs size flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug stats - Accesses stats

Accesses stats

Usage:
   debug stats [flags] <command>

The debug stats commands are:
   read        Returns the value of stats objects.
   watch       Returns a stream of all matching entries and their values as they
               change.

The debug stats flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug stats read

Returns the value of stats objects.

Usage:
   debug stats read [flags] <name> ...

<name> is the name of a stats object, or a glob pattern to match against stats
object names.

The debug stats read flags are:
 -json=false
   When true, the command will display the raw value of the object in json
   format.
 -raw=false
   When true, the command will display the raw value of the object.
 -type=false
   When true, the type of the values will be displayed.

 -timeout=10s
   Time to wait for various RPCs

Debug stats watch

Returns a stream of all matching entries and their values as they change.

Usage:
   debug stats watch [flags] <pattern> ...

<pattern> is a glob pattern to match.

The debug stats watch flags are:
 -raw=false
   When true, the command will display the raw value of the object.
 -type=false
   When true, the type of the values will be displayed.

 -timeout=10s
   Time to wait for various RPCs

Debug pprof - Accesses profiling data

Accesses profiling data

Usage:
   debug pprof [flags] <command>

The debug pprof commands are:
   run         Runs the pprof tool.
   proxy       Runs an http proxy to a pprof object.

The debug pprof flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug pprof run

Runs the pprof tool.

Usage:
   debug pprof run [flags] <name> <profile> [passthru args] ...

<name> is the name of the pprof object. <profile> the name of the profile to
use.

All the [passthru args] are passed to the pprof tool directly, e.g.

  $ debug pprof run a/b/c/__debug/pprof heap --text
  $ debug pprof run a/b/c/__debug/pprof profile -gv

The debug pprof run flags are:
 -pprofcmd=jiri go tool pprof
   The pprof command to use.

 -timeout=10s
   Time to wait for various RPCs

Debug pprof proxy

Runs an http proxy to a pprof object.

Usage:
   debug pprof proxy [flags] <name>

<name> is the name of the pprof object.

The debug pprof proxy flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug browse - Starts an interactive interface for debugging

Starts a webserver with a URL that when visited allows for inspection of a
remote process via a web browser.

This differs from browser.v.io in a few important ways:

  (a) Does not require a chrome extension,
  (b) Is not tied into the v.io cloud services
  (c) Can be setup with alternative different credentials,
  (d) The interface is more geared towards debugging a server than general purpose namespace browsing.

While (d) is easily overcome by sharing code between the two, (a), (b) & (c) are
not easy to work around.  Of course, the down-side here is that this requires
explicit command-line invocation instead of being just a URL anyone can visit
(https://browser.v.io).

A dump of some possible future features: TODO(ashankar):?

  (1) Trace browsing: Browse traces at the remote server, and possible force
  the collection of some traces (avoiding the need to restart the remote server
  with flags like --v23.vtrace.collect-regexp for example). In the mean time,
  use the 'vtrace' command (instead of the 'browse' command) for this purpose.
  (2) Log offsets: Log files can be large and currently the logging endpoint
  of this interface downloads the full log file from the beginning. The ability
  to start looking at the logs only from a specified offset might be useful
  for these large files.
  (3) Signature: Display the interfaces, types etc. defined by any suffix in the
  remote process. in the mean time, use the 'vrpc signature' command for this purpose.

Usage:
   debug browse [flags] <name> [<name>] [<name>]

<name> is the vanadium object name of the remote process to inspect. If multiple
names are provided, they are considered equivalent and any one of them that can
is accessible is used.

The debug browse flags are:
 -addr=
   Address on which the interactive HTTP server will listen. For example,
   localhost:14141. If empty, defaults to localhost:<some random port>
 -assets=
   If non-empty, load assets from this directory.
 -blessings=
   If non-empty, points to the blessings required to debug the process. This is
   typically obtained via 'debug delegate' run by the owner of the remote
   process
 -key=
   If non-empty, must be accompanied with --blessings with a value obtained via
   'debug delegate' run by the owner of the remote process
 -log=true
   If true, log debug data obtained so that if a subsequent refresh from the
   browser fails, previously obtained information is available from the log file

 -timeout=10s
   Time to wait for various RPCs

Debug delegate - Create credentials to delegate debugging to another user

Generates credentials (private key and blessings) required to debug remote
processes owned by the caller and prints out the command that the delegate can
run. The delegation limits the bearer of the token to invoke methods on the
remote process only if they have the "access.Debug" tag on them, and this token
is valid only for limited amount of time. For example, if Alice wants Bob to
debug a remote process owned by her for the next 2 hours, she runs:

  debug delegate my-friend-bob 2h myservices/myservice

And sends Bob the output of this command. Bob will then be able to inspect the
remote process as myservices/myservice with the same authorization as Alice.

Usage:
   debug delegate [flags] <to> <duration> [<name>]

<to> is an identifier to provide to the delegate.

<duration> is the time period to delegate for (e.g., 1h for 1 hour)

<name> (optional) is the vanadium object name of the remote process to inspect

The debug delegate flags are:
 -access=resolve,debug
   Comma-separated list of access tags on methods that can be invoked by the
   delegate

 -timeout=10s
   Time to wait for various RPCs

Debug discovery - Scan and advertise using the v23 discovery API

Usage:
   debug discovery [flags] <command>

The debug discovery commands are:
   scan        Scan for advertised v23 services

The debug discovery flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug discovery scan - Scan for advertised v23 services

Scans for advertised v23 services that match the provided query. The set of
protocols over which services are scanned for depends on the platform on which
this command is being run. As of May 2016, services advertised over mDNS should
be scannable on all platforms but services advertised over BluetoothLE would be
scannable only on OS X.

Usage:
   debug discovery scan [flags] [<query>]

<query> scans for services that match the provided query. An empty query causes
all services to be scanned for. See https://godoc.org/v.io/v23/discovery#T for
details.

Sample queries:
  v.InterfaceName = "v.io/i"
  v.InterfaceName = "v.io/i" AND v.Attributes["a"] = "v"
  v.Attributes["a"] = "v1" OR v.Attributes["a"] = "v2"

The debug discovery scan flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   debug help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The debug help flags are:
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
