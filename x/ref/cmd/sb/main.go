// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help
package main

import (
	"regexp"

	"v.io/x/lib/cmdline"
	"v.io/x/ref/cmd/sb/commands"
	_ "v.io/x/ref/runtime/factories/generic"
)

func main() {
	cmdline.HideGlobalFlagsExcept(
		regexp.MustCompile("^service$"),
		regexp.MustCompile("^db$"),
		regexp.MustCompile("^create-if-absent$"),
		regexp.MustCompile("^format$"),
		regexp.MustCompile("^csv-delimiter$"))
	cmdline.Main(cmdSb)
}

var cmdSb *cmdline.Command

func init() {
	cmdSb = &cmdline.Command{
		Name:  "sb",
		Short: "Vanadium syncbase client and management utility",
		Long: `
Syncbase general-purpose client and management utility.
Supports starting a syncQL shell and executing the commands at top level.
		`,
		Children: append([]*cmdline.Command{cmdSbShell}, commands.Commands...),
	}
}
