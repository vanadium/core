// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package v23cmd implements utilities for running v23 cmdline programs.
//
// The cmdline package requires a Runner that implements Run(env, args), but for
// v23 programs we'd like to implement Run(ctx, env, args) instead.  The
// initialization of the ctx is also tricky, since v23.Init() calls flag.Parse,
// but the cmdline package needs to call flag.Parse first.
//
// The RunnerFunc package-level function allows us to write run functions of the
// form Run(ctx, env, args), retaining static type-safety, and also getting the
// flag.Parse ordering right.
package v23cmd

import (
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
)

type runner struct {
	run  func(*context.T, *cmdline.Env, []string) error
	init func() (*context.T, v23.Shutdown, error)
}

func (r runner) Run(env *cmdline.Env, args []string) error {
	ctx, shutdown, err := r.init()
	if err != nil {
		return err
	}
	defer shutdown()
	return r.run(ctx, env, args)
}

// RunnerFunc is like cmdline.RunnerFunc, but takes a run function that includes
// a context as the first arg.  The context is created via v23.Init when Run is
// called on the returned Runner.
func RunnerFunc(run func(*context.T, *cmdline.Env, []string) error) cmdline.Runner {
	return runner{run, v23.TryInit}
}

// RunnerFuncWithInit is like RunnerFunc, but allows specifying the init
// function used to create the context.
//
// This is typically used to set properties on the context before it is passed
// to the run function.  E.g. you may use this to set a deadline on the context:
//
//   var cmdRoot = &cmdline.Command{
//     Runner: v23cmd.RunnerFuncWithInit(runRoot, initWithDeadline)
//     ...
//   }
//
//   func runRoot(ctx *context.T, env *cmdline.Env, args []string) error {
//     ...
//   }
//
//   func initWithDeadline() (*context.T, v23.Shutdown) {
//     ctx, shutdown := v23.Init()
//     ctx, cancel := context.WithTimeout(ctx, time.Minute)
//     return ctx, func(){ cancel(); shutdown() }
//   }
//
//   func main() {
//     cmdline.Main(cmdRoot)
//   }
//
// An alternative to the above example is to call context.WithTimeout within
// runRoot.  The advantage of using RunnerFuncWithInit is that your regular code
// can use a context with a 1 minute timeout, while your testing code can use
// v23cmd.ParseAndRunForTest to pass a context with a 10 second timeout.
func RunnerFuncWithInit(run func(*context.T, *cmdline.Env, []string) error, init func() (*context.T, v23.Shutdown, error)) cmdline.Runner {
	return runner{run, init}
}

// ParseAndRunForTest parses the cmd with the given env and args, and calls Run
// on the returned runner.  If the runner was created by the v23cmd package,
// calls the run function directly with the given ctx, env and args.
//
// Doesn't call v23.Init; the context initialization is up to you.
//
// Only meant to be called within tests - if used in non-test code the ordering
// of flag.Parse calls will be wrong.  The correct ordering is for cmdline.Parse
// to be called before v23.Init, but this function takes a ctx argument and then
// calls cmdline.Parse.
func ParseAndRunForTest(cmd *cmdline.Command, ctx *context.T, env *cmdline.Env, args []string) error {
	r, args, err := cmdline.Parse(cmd, env, args)
	if err != nil {
		return err
	}
	if x, ok := r.(runner); ok {
		return x.run(ctx, env, args)
	}
	return r.Run(env, args)
}
