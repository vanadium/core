// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v23cmd

import (
	"bytes"
	"fmt"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

var cmdRoot = &cmdline.Command{
	Name:     "root",
	Short:    "root",
	Long:     "root",
	Children: []*cmdline.Command{cmdNoV23, cmdNoInit, cmdWithInit},
}

var cmdNoV23 = &cmdline.Command{
	Name:   "no_v23",
	Short:  "no_v23",
	Long:   "no_v23",
	Runner: cmdline.RunnerFunc(runNoV23),
}

var cmdNoInit = &cmdline.Command{
	Name:   "no_init",
	Short:  "no_init",
	Long:   "no_init",
	Runner: RunnerFunc(runNoInit),
}

var cmdWithInit = &cmdline.Command{
	Name:   "with_init",
	Short:  "with_init",
	Long:   "with_init",
	Runner: RunnerFuncWithInit(runWithInit, initFunc),
}

func runNoV23(env *cmdline.Env, args []string) error {
	fmt.Fprintf(env.Stdout, "NoV23")
	return nil
}

func runNoInit(ctx *context.T, env *cmdline.Env, args []string) error {
	fmt.Fprintf(env.Stdout, "NoInit %v %v", ctx.Value(initKey), ctx.Value(forTestKey))
	return nil
}

func runWithInit(ctx *context.T, env *cmdline.Env, args []string) error {
	fmt.Fprintf(env.Stdout, "WithInit %v %v", ctx.Value(initKey), ctx.Value(forTestKey))
	return nil
}

type ctxKey string

const (
	initKey      ctxKey = "init key"
	initValue    ctxKey = "<init value>"
	forTestKey   ctxKey = "for test key"
	forTestValue ctxKey = "<for test value>"
)

func initFunc() (*context.T, v23.Shutdown, error) {
	ctx, shutdown, err := v23.TryInit()
	return context.WithValue(ctx, initKey, initValue), shutdown, err
}

func TestParseAndRun(t *testing.T) {
	tests := []struct {
		cmd    string
		stdout string
	}{
		{"no_v23", "NoV23"},
		{"no_init", "NoInit <nil> <nil>"},
		{"with_init", "WithInit <init value> <nil>"},
	}
	for _, test := range tests {
		var stdout bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout}
		if err := cmdline.ParseAndRun(cmdRoot, env, []string{test.cmd}); err != nil {
			t.Errorf("ParseAndRun failed: %v", err)
		}
		if got, want := stdout.String(), test.stdout; got != want {
			t.Errorf("got stdout %q, want %q", got, want)
		}
	}
}

func TestParseAndRunForTest(t *testing.T) {
	tests := []struct {
		cmd    string
		stdout string
	}{
		{"no_v23", "NoV23"},
		{"no_init", "NoInit <nil> <for test value>"},
		{"with_init", "WithInit <nil> <for test value>"},
	}
	for _, tc := range tests {
		ctx, shutdown := test.V23Init()
		ctx = context.WithValue(ctx, forTestKey, forTestValue)

		var stdout bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout}
		if err := ParseAndRunForTest(cmdRoot, ctx, env, []string{tc.cmd}); err != nil {
			t.Errorf("ParseAndRunForTest failed: %v", err)
		}
		if got, want := stdout.String(), tc.stdout; got != want {
			t.Errorf("got stdout \"%s\", want \"%s\"", got, want)
		}
		shutdown()
	}
}
