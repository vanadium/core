// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(caprita): Merge this with init.go now that v.io/c/20864 got rid of
// init_mojo.go.

package test

import (
	"flag"
	"os"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/test/testutil"
)

const (
	TestBlessing = "test-blessing"
)

var IntegrationTestsEnabled bool
var IntegrationTestsDebugShellOnError bool

const IntegrationTestsFlag = "v23.tests"
const IntegrationTestsDebugShellOnErrorFlag = "v23.tests.shell-on-fail"

func init() {
	flag.BoolVar(&IntegrationTestsEnabled, IntegrationTestsFlag, false, "Run integration tests.")
	flag.BoolVar(&IntegrationTestsDebugShellOnError, IntegrationTestsDebugShellOnErrorFlag, false, "Drop into a debug shell if an integration test fails.")
}

func editListenSpec(ctx *context.T) *context.T {
	return v23.WithListenSpec(ctx, rpc.ListenSpec{Addrs: rpc.ListenAddrs{{Protocol: "tcp", Address: "127.0.0.1:0"}}})
}

func editPrincipal(ctx *context.T, v23testProcess bool) *context.T {
	if v23testProcess {
		return ctx
	}
	var err error
	if ctx, err = v23.WithPrincipal(ctx, testutil.NewPrincipal(TestBlessing)); err != nil {
		panic(err)
	}
	return ctx
}

func editNamespace(ctx *context.T, v23testProcess, createMounttable bool) {
	ns := v23.GetNamespace(ctx)
	ns.CacheCtl(naming.DisableCache(true))

	// TODO(caprita): Whether this is a Shell child process is but an
	// approximation as to whether we should or should not edit the
	// namespace in the context.  See also TODO on V23Init.
	if v23testProcess {
		return
	}

	if !createMounttable {
		ns.SetRoots() //nolint:errcheck
		return
	}

	// Create a mounttable server and set the namespace root to that.
	disp, err := mounttablelib.NewMountTableDispatcher(ctx, "", "", "mounttable")
	if err != nil {
		panic(err)
	}
	_, s, err := v23.WithNewDispatchingServer(ctx, "", disp, options.ServesMountTable(true))
	if err != nil {
		panic(err)
	}
	ns.SetRoots(s.Status().Endpoints[0].Name()) //nolint:errcheck
}

// internalInit initializes the runtime and returns a new context.
func internalInit(createMounttable bool) (*context.T, func()) {
	ctx, shutdown := v23.Init()
	if *debugOnShutdown != "" {
		orig := shutdown
		shutdown = func() { debug(ctx, orig) }
	}

	v23testProcess := os.Getenv("V23_SHELL_TEST_PROCESS") != ""

	ctx = editListenSpec(ctx)

	ctx = editPrincipal(ctx, v23testProcess)

	editNamespace(ctx, v23testProcess, createMounttable)

	return ctx, shutdown
}

// TestContext returns a *context.T suitable for use in tests. It sets the
// context's logger to logger.Global(), and that's it. In particular, it does
// not call v23.Init().
func TestContext() (*context.T, context.CancelFunc) {
	ctx, cancel := context.RootContext()
	return context.WithLogger(ctx, logger.Global()), cancel
}
