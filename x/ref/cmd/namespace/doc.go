// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command namespace resolves and manages names in the Vanadium namespace.

The namespace roots are set from the command line via --v23.namespace.root
command line option or from environment variables that have a name starting with
V23_NAMESPACE, e.g.  V23_NAMESPACE, V23_NAMESPACE_2, V23_NAMESPACE_GOOGLE, etc.
The command line options override the environment.

Usage:
   namespace [flags] <command>

The namespace commands are:
   glob        Returns all matching entries from the namespace
   mount       Adds a server to the namespace
   unmount     Removes a server from the namespace
   resolve     Translates a object name to its object address(es)
   resolvetomt Finds the address of the mounttable that holds an object name
   permissions Manipulates permissions on an entry in the namespace
   delete      Deletes a name from the namespace
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

Namespace glob - Returns all matching entries from the namespace

Returns all matching entries from the namespace.

Usage:
   namespace glob [flags] <pattern>

<pattern> is a glob pattern that is matched against all the names below the
specified mount name.

The namespace glob flags are:
 -fail-on-any-error=false
   exit on any error, by default glob will report errors but not return a
   non-zero exit code
 -l=false
   Long listing format.

Namespace mount - Adds a server to the namespace

Adds server <server> to the namespace with name <name>.

Usage:
   namespace mount [flags] <name> <server> <ttl>

<name> is the name to add to the namespace. <server> is the object address of
the server to add. <ttl> is the TTL of the new entry. It is a decimal number
followed by a unit suffix (s, m, h). A value of 0s represents an infinite
duration.

Namespace unmount - Removes a server from the namespace

Removes server <server> with name <name> from the namespace.

Usage:
   namespace unmount [flags] <name> <server>

<name> is the name to remove from the namespace. <server> is the object address
of the server to remove.

Namespace resolve

Translates a object name to its object address(es).

Usage:
   namespace resolve [flags] <name>

<name> is the name to resolve.

The namespace resolve flags are:
 -insecure=false
   Insecure mode: May return results from untrusted servers and invoke Resolve
   on untrusted mounttables
 -s=false
   True to perform a shallow resolution

Namespace resolvetomt - Finds the address of the mounttable that holds an object name

Finds the address of the mounttable that holds an object name.

Usage:
   namespace resolvetomt [flags] <name>

<name> is the name to resolve.

The namespace resolvetomt flags are:
 -insecure=false
   Insecure mode: May return results from untrusted servers and invoke Resolve
   on untrusted mounttables

Namespace permissions - Manipulates permissions on an entry in the namespace

Commands to get and set the permissions on a name - controlling the blessing
names required to resolve the name.

The permissions are provided as an JSON-encoded version of the Permissions type
defined in v.io/v23/security/access/types.vdl.

Usage:
   namespace permissions [flags] <command>

The namespace permissions commands are:
   get         Gets permissions on a mount name
   set         Sets permissions on a mount name

Namespace permissions get - Gets permissions on a mount name

Get retrieves the permissions on the usage of a name.

The output is a JSON-encoded Permissions object (defined in
v.io/v23/security/access/types.vdl).

Usage:
   namespace permissions get [flags] <name>

<name> is a name in the namespace.

Namespace permissions set - Sets permissions on a mount name

Set replaces the permissions controlling usage of a mount name.

Usage:
   namespace permissions set [flags] <name> <permissions>

<name> is the name on which permissions are to be set.

<permissions> is the path to a file containing a JSON-encoded Permissions object
(defined in v.io/v23/security/access/types.vdl), or "-" for STDIN.

Namespace delete - Deletes a name from the namespace

Deletes a name from the namespace.

Usage:
   namespace delete [flags] <name>

<name> is a name to delete.

The namespace delete flags are:
 -r=false
   Delete all children of the name in addition to the name itself.

Namespace help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   namespace help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The namespace help flags are:
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
