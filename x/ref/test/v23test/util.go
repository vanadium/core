// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines helper functions for running specific Vanadium binaries
// using v23shell.Shell.

package v23test

import (
	"errors"
	"io"
	"os"

	v23 "v.io/v23"
	"v.io/x/lib/gosh"
	"v.io/x/ref"
	"v.io/x/ref/services/mounttable/mounttablelib"
)

// TODO(sadovsky): Drop this hack once the TODOs in v23test.go are addressed.
func maybeAddTCPAddressFlag(sh *Shell, args *[]string) {
	if _, ok := sh.Vars[envShellTestProcess]; ok {
		*args = append(*args, "-v23.tcp.address=127.0.0.1:0")
	}
}

var mounttabledMain = gosh.RegisterFunc("mounttabledMain", mounttablelib.Main)

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
	maybeAddTCPAddressFlag(sh, &args)
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

// Copied over from gosh.
//nolint:deadcode,unused
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
