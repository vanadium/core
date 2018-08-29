// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command uniqueid generates unique identifiers. It also has an option of
automatically substituting unique ids with placeholders in files.

Usage:
   uniqueid [flags] <command>

The uniqueid commands are:
   generate    Generates UniqueIds
   inject      Injects UniqueIds into existing files
   help        Display help for commands or topics

The global flags are:
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -time=false
   Dump timing information to stderr before exiting the program.

Uniqueid generate - Generates UniqueIds

Generates unique ids and outputs them to standard out.

Usage:
   uniqueid generate [flags]

Uniqueid inject - Injects UniqueIds into existing files

Injects UniqueIds into existing files. Strings of the form "$UNIQUEID$" will be
replaced with generated ids.

Usage:
   uniqueid inject [flags] <filenames>

<filenames> List of files to inject unique ids into

Uniqueid help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   uniqueid help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The uniqueid help flags are:
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
