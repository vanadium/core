// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command vdl manages Vanadium Definition Language source code.  It's similar to
the go tool used for managing Go source code.

Usage:
   vdl [flags] <command>

The vdl commands are:
   generate    Compile packages and dependencies, and generate code
   compile     Compile packages and dependencies, but don't generate code
   audit       Check if any packages are stale and need generation
   list        List package and dependency info in transitive order
   help        Display help for commands or topics

The vdl additional help topics are:
   packages    Description of package lists
   vdlpath     Description of VDLPATH environment variable
   vdlroot     Description of VDLROOT environment variable
   vdl.config  Description of vdl.config files

The vdl flags are:
 -ignore_unknown=false
   Ignore unknown packages provided on the command line.
 -max-errors=-1
   Stop processing after this many errors, or -1 for unlimited.
 -v=false
   Turn on verbose logging.
 -vdl.config=vdl.config
   Basename of the optional per-package config file.

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
 -skip-go-methods=false
   Skip go generation of VDL{Read,Write,Zero} methods.
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

Vdl generate - Compile packages and dependencies, and generate code

Generate compiles packages and their transitive dependencies, and generates code
in the specified languages.

Usage:
   vdl generate [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The vdl generate flags are:
 -errors-no-i18n=false
   No longer support i18n formats for errors
 -go-out-dir=
   Go output directory.  There are three modes:
      ""                     : Generate output in-place in the source tree
      "dir"                  : Generate output rooted at dir
      "src->dst[,s2->d2...]" : Generate output using translation rules
   Assume your source tree is organized as follows:
      VDLPATH=/home/vdl
         /home/vdl/test_base/base1.vdl
         /home/vdl/test_base/base2.vdl
   Here's example output under the different modes:
      --go-out-dir=""
         /home/vdl/test_base/base1.vdl.go
         /home/vdl/test_base/base2.vdl.go
      --go-out-dir="/tmp/foo"
         /tmp/foo/test_base/base1.vdl.go
         /tmp/foo/test_base/base2.vdl.go
      --go-out-dir="vdl->foo/bar"
         /home/foo/bar/test_base/base1.vdl.go
         /home/foo/bar/test_base/base2.vdl.go
   When the src->dst form is used, src must match the suffix of the path just
   before the package path, and dst is the replacement for src.  Use commas to
   separate multiple rules; the first rule matching src is used.  The special
   dst SKIP indicates matching packages are skipped.
 -java-out-dir=release/go/src->release/java/lib/generated-src/vdl,roadmap/go/src->release/java/lib/generated-src/vdl
   Same semantics as --go-out-dir but applies to java code generation.
 -java-out-pkg=v.io->io/v
   Java output package translation rules.  Must be of the form:
      "src->dst[,s2->d2...]"
   If a VDL package has a prefix src, the prefix will be replaced with dst.  Use
   commas to separate multiple rules; the first rule matching src is used, and
   if there are no matching rules, the package remains unchanged.  The special
   dst SKIP indicates those packages containing the string are skipped. Note
   this skip behavior is slightly different than the -out-dir semantics which is
   prefix-based.
 -js-out-dir=release/go/src->release/javascript/core/src,roadmap/go/src->release/javascript/core/src,third_party/go/src->SKIP,tools/go/src->SKIP,release/go/src/v.io/v23/vdlroot->SKIP
   Same semantics as --go-out-dir but applies to js code generation.
 -js-relative-path-to-core=
   If set, this is the relative path from js-out-dir to the root of the JS core
 -lang=Go
   Comma-separated list of languages to generate, currently supporting
   Go,Java,Javascript,Swift
 -show-warnings=true
   show warning messages
 -status=true
   Show package names as they are updated
 -swift-out-dir=release/go/src->release/swift/lib/generated-src/vdl,roadmap/go/src->release/swift/lib/generated-src/vdl
   Same semantics as --go-out-dir but applies to Swift code generation.
 -swift-out-pkg=v.io/x->SKIP,/internal->SKIP,/testdata->SKIP,v.io/v23->
   Swift output package translation rules.  Must be of the form:
      "src->dst[,s2->d2...]"
   If a VDL package has a prefix src, the prefix will be replaced with dst.  Use
   commas to separate multiple rules; the first rule matching src is used, and
   if there are no matching rules, the package remains unchanged.  The special
   dst SKIP indicates those packages containing the string are skipped. Note
   this skip behavior is slightly different than the -out-dir semantics which is
   prefix-based.

 -ignore_unknown=false
   Ignore unknown packages provided on the command line.
 -max-errors=-1
   Stop processing after this many errors, or -1 for unlimited.
 -v=false
   Turn on verbose logging.
 -vdl.config=vdl.config
   Basename of the optional per-package config file.

Vdl compile

Compile compiles packages and their transitive dependencies, but does not
generate code.  This is useful to sanity-check that your VDL files are valid.

Usage:
   vdl compile [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The vdl compile flags are:
 -errors-no-i18n=false
   No longer support i18n formats for errors
 -show-warnings=true
   show warning messages
 -status=true
   Show package names while we compile

 -ignore_unknown=false
   Ignore unknown packages provided on the command line.
 -max-errors=-1
   Stop processing after this many errors, or -1 for unlimited.
 -v=false
   Turn on verbose logging.
 -vdl.config=vdl.config
   Basename of the optional per-package config file.

Vdl audit - Check if any packages are stale and need generation

Audit runs the same logic as generate, but doesn't write out generated files.
Returns a 0 exit code if all packages are up-to-date, otherwise returns a non-0
exit code indicating some packages need generation.

Usage:
   vdl audit [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The vdl audit flags are:
 -errors-no-i18n=false
   No longer support i18n formats for errors
 -go-out-dir=
   Go output directory.  There are three modes:
      ""                     : Generate output in-place in the source tree
      "dir"                  : Generate output rooted at dir
      "src->dst[,s2->d2...]" : Generate output using translation rules
   Assume your source tree is organized as follows:
      VDLPATH=/home/vdl
         /home/vdl/test_base/base1.vdl
         /home/vdl/test_base/base2.vdl
   Here's example output under the different modes:
      --go-out-dir=""
         /home/vdl/test_base/base1.vdl.go
         /home/vdl/test_base/base2.vdl.go
      --go-out-dir="/tmp/foo"
         /tmp/foo/test_base/base1.vdl.go
         /tmp/foo/test_base/base2.vdl.go
      --go-out-dir="vdl->foo/bar"
         /home/foo/bar/test_base/base1.vdl.go
         /home/foo/bar/test_base/base2.vdl.go
   When the src->dst form is used, src must match the suffix of the path just
   before the package path, and dst is the replacement for src.  Use commas to
   separate multiple rules; the first rule matching src is used.  The special
   dst SKIP indicates matching packages are skipped.
 -java-out-dir=release/go/src->release/java/lib/generated-src/vdl,roadmap/go/src->release/java/lib/generated-src/vdl
   Same semantics as --go-out-dir but applies to java code generation.
 -java-out-pkg=v.io->io/v
   Java output package translation rules.  Must be of the form:
      "src->dst[,s2->d2...]"
   If a VDL package has a prefix src, the prefix will be replaced with dst.  Use
   commas to separate multiple rules; the first rule matching src is used, and
   if there are no matching rules, the package remains unchanged.  The special
   dst SKIP indicates those packages containing the string are skipped. Note
   this skip behavior is slightly different than the -out-dir semantics which is
   prefix-based.
 -js-out-dir=release/go/src->release/javascript/core/src,roadmap/go/src->release/javascript/core/src,third_party/go/src->SKIP,tools/go/src->SKIP,release/go/src/v.io/v23/vdlroot->SKIP
   Same semantics as --go-out-dir but applies to js code generation.
 -js-relative-path-to-core=
   If set, this is the relative path from js-out-dir to the root of the JS core
 -lang=Go
   Comma-separated list of languages to generate, currently supporting
   Go,Java,Javascript,Swift
 -show-warnings=true
   show warning messages
 -status=true
   Show package names as they are updated
 -swift-out-dir=release/go/src->release/swift/lib/generated-src/vdl,roadmap/go/src->release/swift/lib/generated-src/vdl
   Same semantics as --go-out-dir but applies to Swift code generation.
 -swift-out-pkg=v.io/x->SKIP,/internal->SKIP,/testdata->SKIP,v.io/v23->
   Swift output package translation rules.  Must be of the form:
      "src->dst[,s2->d2...]"
   If a VDL package has a prefix src, the prefix will be replaced with dst.  Use
   commas to separate multiple rules; the first rule matching src is used, and
   if there are no matching rules, the package remains unchanged.  The special
   dst SKIP indicates those packages containing the string are skipped. Note
   this skip behavior is slightly different than the -out-dir semantics which is
   prefix-based.

 -ignore_unknown=false
   Ignore unknown packages provided on the command line.
 -max-errors=-1
   Stop processing after this many errors, or -1 for unlimited.
 -v=false
   Turn on verbose logging.
 -vdl.config=vdl.config
   Basename of the optional per-package config file.

Vdl list - List package and dependency info in transitive order

List returns information about packages and their transitive dependencies, in
transitive order.  This is the same order the generate and compile commands use
for processing.  If "vdl list A" is run and A depends on B, which depends on C,
the returned order will be C, B, A.  If multiple packages are specified the
ordering is over all combined dependencies.

Reminder: cyclic dependencies between packages are not allowed.  Cyclic
dependencies between VDL files within the same package are also not allowed.
This is more strict than regular Go; it makes it easier to generate code for
other languages like C++.

Usage:
   vdl list [flags] <packages>

<packages> are a list of packages to process, similar to the standard go tool.
For more information, run "vdl help packages".

The vdl list flags are:
 -ignore_unknown=false
   Ignore unknown packages provided on the command line.
 -max-errors=-1
   Stop processing after this many errors, or -1 for unlimited.
 -v=false
   Turn on verbose logging.
 -vdl.config=vdl.config
   Basename of the optional per-package config file.

Vdl help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   vdl help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The vdl help flags are:
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

Vdl packages - Description of package lists

Most vdl commands apply to a list of packages:

   vdl command <packages>

<packages> are a list of packages to process, similar to the standard go tool.
In its simplest form each package is an import path; e.g.
   "v.io/x/ref/lib/vdl"

A package that is an absolute path or that begins with a . or .. element is
interpreted as a file system path, and denotes the package in that directory.

A package is a pattern if it includes one or more "..." wildcards, each of which
can match any string, including the empty string and strings containing slashes.
Such a pattern expands to all packages found in VDLPATH with names matching the
pattern.  As a special-case, x/... matches x as well as x's subdirectories.

The special-case "all" is a synonym for "...", and denotes all packages found in
VDLPATH.

Import path elements and file names are not allowed to begin with "." or "_";
such paths are ignored in wildcard matches, and return errors if specified
explicitly.

VDLPATH requires *.vdl source files and packages to appear directly under the
VDLPATH directories. Note that when go modules are used VDLPATH should point to
the location of the go.mod file. Also note that whereas GOPATH requires *.go
source files and packages to appear under a "src" directory, VDLPATH requires
*.vdl source files and packages to appear directly under the VDLPATH
directories.

 Run "vdl help vdlpath" to see docs on VDLPATH.
 Run "go help packages" to see the standard go package docs.

Vdl vdlpath - Description of VDLPATH environment variable

The VDLPATH environment variable is used to resolve import statements. It must
be set to compile and generate vdl packages.

The format is a colon-separated list of directories containing vdl source code.
These directories are searched recursively for VDL source files.  The path below
the directory determines the import path.  If VDLPATH specifies multiple
directories, imports are resolved by picking the first directory with a matching
import name.

An example:

   VDPATH=/home/user/vdlA:/home/user/vdlB

   /home/user/vdlA
       foo/                 (import "foo" refers here)
           foo1.vdl
   /home/user/vdlB
       foo/                 (this package is ignored)
          foo2.vdl
       bar/
          baz/              (import "bar/baz" refers here)
             baz.vdl

Vdl vdlroot - Description of VDLROOT environment variable

The VDLROOT environment variable is similar to VDLPATH, but instead of pointing
to multiple user source directories, it points at a single source directory
containing the standard vdl packages.

If VDLROOT is empty, we use the standard packages built-in to the vdl binary.
VDLROOT is typically left empty, except by vdl tool developers.

Vdl vdl.config - Description of vdl.config files

Each vdl source package P may contain an optional file "vdl.config" within the P
directory.  This file specifies additional configuration for the vdl tool.

The format of this file is described by the vdltool.Config type in the "vdltool"
standard package, located at VDLROOT/vdltool/config.vdl.

If the file does not exist, we use the zero value of vdl.Config.
*/
package main
