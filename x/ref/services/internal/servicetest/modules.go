// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package servicetest

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/options"
	"v.io/x/lib/gosh"
	"v.io/x/ref"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/services/internal/dirprinter"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23test"
)

const (
	// Setting this environment variable to any non-empty value avoids
	// removing the generated workspace for successful test runs (for
	// failed test runs, this is already the case).  This is useful when
	// developing test cases.
	preserveWorkspaceEnv = "V23_TEST_PRESERVE_WORKSPACE"
)

var rootMT = gosh.RegisterFunc("rootMT", func() error {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	mt, err := mounttablelib.NewMountTableDispatcher(ctx, "", "", "mounttable")
	if err != nil {
		return fmt.Errorf("mounttablelib.NewMountTableDispatcher failed: %s", err)
	}
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", mt, options.ServesMountTable(true))
	if err != nil {
		return fmt.Errorf("rootMT failed: %v", err)
	}
	gosh.SendVars(map[string]string{"NAME": server.Status().Endpoints[0].Name()})
	<-signals.ShutdownOnSignals(ctx)
	return nil
})

// startRootMT sets up a root mount table for tests.
func startRootMT(t *testing.T, sh *v23test.Shell) string {
	c := sh.FuncCmd(rootMT)
	c.Args = append(c.Args, "--v23.tcp.address=127.0.0.1:0")
	c.PropagateOutput = false
	c.Start()
	return c.AwaitVars("NAME")["NAME"]
}

// setNSRoots sets the roots for the local runtime's namespace.
func setNSRoots(t *testing.T, ctx *context.T, roots ...string) {
	ns := v23.GetNamespace(ctx)
	if err := ns.SetRoots(roots...); err != nil {
		t.Fatal(testutil.FormatLogLine(3, "SetRoots(%v) failed: %v", roots, err))
	}
}

// CreateShellAndMountTable builds a new shell and starts a root mount table.
// Returns the shell and a cleanup function.
// TODO(sadovsky): Use v23test.StartRootMountTable.
func CreateShellAndMountTable(t *testing.T, ctx *context.T) (*v23test.Shell, func()) {
	sh := v23test.NewShell(t, ctx)
	sh.PropagateChildOutput = true
	ok := false
	defer func() {
		if !ok {
			sh.Cleanup()
		}
	}()
	mtName := startRootMT(t, sh)
	ctx.VI(1).Infof("Started root mount table with name %s", mtName)
	oldNamespaceRoots := v23.GetNamespace(ctx).Roots()
	fn := func() {
		ctx.VI(1).Info("------------ CLEANUP ------------")
		ctx.VI(1).Info("---------------------------------")
		ctx.VI(1).Info("--(cleaning up shell)------------")
		sh.Cleanup()
		ctx.VI(1).Info("--(done cleaning up shell)-------")
		setNSRoots(t, ctx, oldNamespaceRoots...)
	}
	setNSRoots(t, ctx, mtName)
	sh.Vars[ref.EnvNamespacePrefix] = mtName
	ok = true
	return sh, fn
}

// CreateShell builds a new shell.
// Returns the shell and a cleanup function.
func CreateShell(t *testing.T, ctx *context.T) (*v23test.Shell, func()) {
	sh := v23test.NewShell(t, ctx)
	sh.PropagateChildOutput = true
	ok := false
	defer func() {
		if !ok {
			sh.Cleanup()
		}
	}()
	fn := func() {
		ctx.VI(1).Info("------------ CLEANUP ------------")
		ctx.VI(1).Info("---------------------------------")
		ctx.VI(1).Info("--(cleaning up shell)------------")
		sh.Cleanup()
		ctx.VI(1).Info("--(done cleaning up shell)-------")
	}
	nsRoots := v23.GetNamespace(ctx).Roots()
	if len(nsRoots) == 0 {
		t.Fatalf("no namespace roots")
	}
	sh.Vars[ref.EnvNamespacePrefix] = nsRoots[0]
	ok = true
	return sh, fn
}

// SetupRootDir sets up and returns a directory for the root and returns
// a cleanup function.
func SetupRootDir(t *testing.T, prefix string) (string, func()) {
	rootDir, err := ioutil.TempDir("", prefix)
	if err != nil {
		t.Fatalf("Failed to set up temporary dir for test: %v", err)
	}
	// On some operating systems (e.g. darwin) os.TempDir() can return a
	// symlink. To avoid having to account for this eventuality later,
	// evaluate the symlink.
	rootDir, err = filepath.EvalSymlinks(rootDir)
	if err != nil {
		logger.Global().Fatalf("EvalSymlinks(%v) failed: %v", rootDir, err)
	}
	return rootDir, func() {
		if t.Failed() || os.Getenv(preserveWorkspaceEnv) != "" {
			t.Logf("A dump of the %s workspace at %v:", prefix, rootDir)
			var dump bytes.Buffer
			if err := dirprinter.DumpDir(&dump, rootDir); err != nil {
				t.Logf("Failed to dump %v: %v", rootDir, err)
			} else {
				t.Log("\n" + dump.String())
			}
		} else {
			os.RemoveAll(rootDir)
		}
	}
}
