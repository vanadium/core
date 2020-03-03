// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command vom helps debug the Vanadium Object Marshaling wire protocol.

Usage:
   vom [flags] <command>

The vom commands are:
   decode      Decode data encoded in the vom format
   dump        Dump data encoded in the vom format into formatted output
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
 -vmodule=
   comma-separated list of globpattern=N settings for filename-filtered logging
   (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns baz or
   *az or b* but not by bar/baz or baz.go or az or b.*
 -vpath=
   comma-separated list of regexppattern=N settings for file pathname-filtered
   logging (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns
   foo/bar/baz or fo.*az or oo/ba or b.z but not by foo/bar/baz.go or fo*az

Vom decode - Decode data encoded in the vom format

Decode decodes data encoded in the vom format.  If no arguments are provided,
decode reads the data from stdin, otherwise the argument is the data.

By default the data is assumed to be represented in hex, with all whitespace
anywhere in the data ignored.  Use the -data flag to specify other data
representations.

Usage:
   vom decode [flags] [data]

[data] is the data to decode; if not specified, reads from stdin

The vom decode flags are:
 -data=Hex
   Data representation, one of [Hex Binary]

Vom dump - Dump data encoded in the vom format into formatted output

Dump dumps data encoded in the vom format, generating formatted output
describing each portion of the encoding.  If no arguments are provided, dump
reads the data from stdin, otherwise the argument is the data.

By default the data is assumed to be represented in hex, with all whitespace
anywhere in the data ignored.  Use the -data flag to specify other data
representations.

Calling "vom dump" with no flags and no arguments combines the default stdin
mode with the default hex mode.  This default mode is special; certain non-hex
characters may be input to represent commands:
  . (period)    Calls Dumper.Status to get the current decoding status.
  ; (semicolon) Calls Dumper.Flush to flush output and start a new message.

This lets you cut-and-paste hex strings into your terminal, and use the commands
to trigger status or flush calls; i.e. a rudimentary debugging UI.

See v.io/v23/vom.Dumper for details on the dump output.

Usage:
   vom dump [flags] [data]

[data] is the data to dump; if not specified, reads from stdin

The vom dump flags are:
 -data=Hex
   Data representation, one of [Hex Binary]

Vom help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   vom help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The vom help flags are:
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
