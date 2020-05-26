// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"errors"
	"fmt"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
)

var name string

func main() {
	cmdHelloClient.Flags.StringVar(&name, "name", "", "Name of the hello server.")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdHelloClient)
}

var cmdHelloClient = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runHelloClient),
	Name:   "helloclient",
	Short:  "Simple client mainly used in regression tests.",
	Long: `
Command helloclient is a simple client mainly used in regression tests.
`,
}

func runHelloClient(ctx *context.T, env *cmdline.Env, args []string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var result string
	err := v23.GetClient(ctx).Call(ctx, name, "Hello", nil, []interface{}{&result})
	if err != nil {
		return errors.New(verror.DebugString(err))
	}

	if got, want := result, "hello"; got != want {
		return fmt.Errorf("Unexpected result, got %q, want %q", got, want)
	}
	return nil
}
