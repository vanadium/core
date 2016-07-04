// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines helper functions for running specific Vanadium binaries
// using v23shell.Shell.

package v23test

import (
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"v.io/v23"
	"v.io/x/lib/gosh"
	"v.io/x/ref"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/services/syncbase/syncbaselib"
)

var syncbaseDebugArgs = flag.String("v23test-syncbase-debug-args", "", "args to add to syncbased invocations; if non-empty, a -log_dir will be created automatically for each invocation")

// TODO(sadovsky): Drop this hack once the TODOs in v23test.go are addressed.
func maybeAddTcpAddressFlag(sh *Shell, args *[]string) {
	if _, ok := sh.Vars[envShellTestProcess]; ok {
		*args = append(*args, "-v23.tcp.address=127.0.0.1:0")
	}
}

var mounttabledMain = gosh.RegisterFunc("mounttabledMain", mounttablelib.Main)
var syncbasedMain = gosh.RegisterFunc("syncbasedMain", syncbaselib.Main)

// StartRootMountTable calls StartRootMountTableWithOpts with default options.
func (sh *Shell) StartRootMountTable() func(sig os.Signal) {
	return sh.StartRootMountTableWithOpts(mounttablelib.Opts{})
}

// StartRootMountTableWithOpts starts mounttabled and calls SetRoots. Returns a
// function that can be called to send a signal to the started process and wait
// for it to exit.
func (sh *Shell) StartRootMountTableWithOpts(opts mounttablelib.Opts) func(sig os.Signal) {
	sh.Ok()
	cmd := sh.FuncCmd(mounttabledMain, opts)
	if sh.Err != nil {
		return nil
	}
	args := []string{}
	maybeAddTcpAddressFlag(sh, &args)
	cmd.Args = append(cmd.Args, args...)
	cmd.Start()
	if sh.Err != nil {
		return nil
	}
	name := cmd.S.ExpectVar("NAME")
	if name == "" {
		sh.handleError(errors.New("v23test: mounttabled failed to start"))
		return nil
	}
	sh.Vars[ref.EnvNamespacePrefix] = name
	if err := v23.GetNamespace(sh.Ctx).SetRoots(name); err != nil {
		sh.handleError(err)
		return nil
	}
	sh.Ctx.Infof("Started root mount table: %s", name)
	return cmd.Terminate
}

// StartSyncbase builds and starts syncbased. If opts.RootDir is empty, it makes
// a new root dir. Returns a function that can be called to send a signal to the
// started process and wait for it to exit.
// TODO(sadovsky): Maybe take a Permissions object instead of permsLiteral.
func (sh *Shell) StartSyncbase(c *Credentials, opts syncbaselib.Opts, permsLiteral string) func(sig os.Signal) {
	sh.Ok()

	args := []string{}
	var syncbaseLogDir string
	if *syncbaseDebugArgs != "" {
		args = append(args, strings.Fields(*syncbaseDebugArgs)...)

		var err error
		syncbaseLogDir, err = ioutil.TempDir("", opts.Name+"-")
		if err != nil {
			sh.handleError(err)
			return nil
		}
		args = append(args, "-log_dir="+syncbaseLogDir)
		sh.Ctx.Infof("syncbased -log_dir for %s: %s", opts.Name, syncbaseLogDir)

		cpuProfileFile, err := ioutil.TempFile(syncbaseLogDir, "cpu-"+opts.Name+"-")
		if err != nil {
			sh.handleError(err)
			return nil
		}
		opts.CpuProfile = cpuProfileFile.Name()
		sh.Ctx.Infof("syncbased -cpuprofile for %s: %s", opts.Name, opts.CpuProfile)
	}

	cmd := sh.FuncCmd(syncbasedMain, opts)
	if sh.Err != nil {
		return nil
	}
	maybeAddTcpAddressFlag(sh, &args)
	args = append(args, "-v23.permissions.literal="+permsLiteral)
	cmd.Args = append(cmd.Args, args...)
	if c != nil {
		cmd = cmd.WithCredentials(c)
	}

	if *syncbaseDebugArgs != "" {
		// Copy executable into syncbaseLogDir, for use by pprof.
		if err := copyFile(syncbaseLogDir+"/syncbased", cmd.Args[0]); err != nil {
			sh.handleError(err)
			return nil
		}
	}

	cmd.Start()
	endpoint := cmd.S.ExpectVar("ENDPOINT")
	if endpoint == "" {
		sh.handleError(errors.New("v23test: syncbased failed to start"))
		return nil
	}
	sh.Ctx.Infof("Started syncbase: %s", endpoint)
	return cmd.Terminate
}

// Copied over from gosh.
func copyFile(to, from string) error {
	fi, err := os.Stat(from)
	if err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}
