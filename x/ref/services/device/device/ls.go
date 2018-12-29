// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"

	"v.io/v23/context"

	"v.io/x/lib/cmdline"
)

var cmdLs = &cmdline.Command{
	Name:     "ls",
	Short:    "List applications.",
	Long:     "List application installations or instances.",
	ArgsName: "<app name patterns...>",
	ArgsLong: `
<app name patterns...> are vanadium object names or glob name patterns corresponding to application installations and instances.`,
}

func init() {
	globify(cmdLs, runLs, new(GlobSettings))
}

func runLs(entry GlobResult, _ *context.T, stdout, _ io.Writer) error {
	fmt.Fprintf(stdout, "%v\n", entry.Name)
	return nil
}
