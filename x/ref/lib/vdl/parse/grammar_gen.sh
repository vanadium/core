#!/bin/bash
# Copyright 2015 The Vanadium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Generate the grammar.go source file, which contains the parser, by running
# this shell script in the same directory, or by running go generate.  This also
# generates grammar.y.debug, which contains a list of all states produced for
# the parser, and some stats.

set -e

go get "golang.org/x/tools/cmd/goyacc"
go run "golang.org/x/tools/cmd/goyacc" -o grammar.y.tmp.go -v grammar.y.debug.tmp grammar.y
gofmt -l -w grammar.y.tmp.go
cat - grammar.y.tmp.go > grammar.y.go <<EOF
// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

EOF
cat - grammar.y.debug.tmp > grammar.y.debug <<EOF
***** PLEASE READ THIS! DO NOT DELETE THIS BLOCK! *****
* The main reason this file has been generated and submitted is to try to ensure
* we never submit changes that cause shift/reduce or reduce/reduce conflicts.
* The Go yacc tool doesn't support the %expect directive, and will happily
* generate a parser even if such conflicts exist; it's up to the developer
* running the tool to notice that an error message is reported.  The bottom of
* this file contains stats, including the number of conflicts.  If you're
* reviewing a change make sure it says 0 conflicts.
*
* If you're updating the grammar, just cut-and-paste this message from the old
* file to the new one, so that this comment block persists.
***** PLEASE READ THIS! DO NOT DELETE THIS BLOCK! *****
EOF

rm grammar.y.debug.tmp grammar.y.tmp.go
