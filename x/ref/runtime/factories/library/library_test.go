// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package library_test

import (
	"reflect"
	"strings"
	"testing"

	"v.io/x/lib/gosh"

	"v.io/x/ref/lib/flags"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/x/ref/runtime/factories/library"
)

func init() {
	library.AllowMultipleInitializations = true
}

func getValues(ctx *context.T) (rpc.ListenSpec, access.PermissionsSpec, []string) {
	return v23.GetListenSpec(ctx),
		v23.GetPermissionsSpec(ctx),
		v23.GetNamespace(ctx).Roots()
}

func TestStatic(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()
	ls, ps, roots := getValues(ctx)
	if got, want := ls.String(), `("wsh", ":0")`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := ps.Literal, ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	empty := map[string]string{}
	if got, want := ps.Files, empty; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := roots, flags.DefaultNamespaceRootsNoEnv(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCommandLineFlagDefaults(t *testing.T) {
	oldProtocol := flags.DefaultProtocol()
	oldHostport := flags.DefaultHostPort()
	oldProxy := flags.DefaultProxy()
	oldRoots := flags.DefaultNamespaceRootsNoEnv()
	oldPerms := flags.DefaultPermissions()
	oldLiteral := flags.DefaultPermissionsLiteral()
	oldPermSpec := library.ConfigurePermissionsFromFlags
	defer func() {
		flags.SetDefaultProtocol(oldProtocol)
		flags.SetDefaultHostPort(oldHostport)
		flags.SetDefaultProxy(oldProxy)
		flags.SetDefaultNamespaceRoots(oldRoots...)
		flags.SetDefaultPermissionsLiteral(oldLiteral)
		for k, v := range oldPerms {
			flags.SetDefaultPermissions(k, v)
		}
		library.ConfigurePermissionsFromFlags = oldPermSpec
	}()
	flags.SetDefaultProtocol("tcp6")
	flags.SetDefaultHostPort("127.0.0.2:9999")
	flags.SetDefaultNamespaceRoots("/myroot")
	flags.SetDefaultProxy("myrpoxy")
	flags.SetDefaultPermissions("a", "b")
	flags.SetDefaultPermissions("c", "d")
	flags.SetDefaultPermissionsLiteral("{ohmy}")
	library.ConfigurePermissionsFromFlags = true

	ctx, shutdown := v23.Init()
	defer shutdown()

	ls, ps, roots := getValues(ctx)

	if got, want := ls.String(), `("tcp6", "127.0.0.2:9999") proxy(myrpoxy)`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := ps.Literal, "{ohmy}"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	perms := map[string]string{"a": "b", "c": "d"}
	if got, want := ps.Files, perms; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := roots, []string{"/myroot"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFlagParsing(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()
	sh.ContinueOnError = true
	cmd := sh.Cmd("go",
		"run",
		"v.io/x/ref/runtime/factories/library/internal/noflags",
		"--help")
	stdout, stderr := cmd.StdoutStderr()

	if got, want := stdout, "Principal"; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}
	if got, want := stderr, ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	cmd = sh.Cmd("go",
		"run",
		"v.io/x/ref/runtime/factories/library/internal/withflags",
		"--help")
	stdout, stderr = cmd.StdoutStderr()

	if got, want := stdout, ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := stderr, "Usage"; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}
}

func TestPreParsing(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()
	sh.ContinueOnError = true
	cmd := sh.Cmd("go",
		"run",
		"v.io/x/ref/runtime/factories/library/internal/preparsed",
		"--help")
	_, stderr := cmd.StdoutStderr()
	if got, want := stderr, "exit status 10\n"; !strings.HasSuffix(got, want) {
		t.Errorf("got %v does end with %v", got, want)
	}
	sh.Err = nil
	cmd = sh.Cmd("go",
		"run",
		"v.io/x/ref/runtime/factories/library/internal/notpreparsed",
		"--help")
	_, stderr = cmd.StdoutStderr()
	if got, want := stderr, "exit status 1\n"; !strings.HasSuffix(got, want) {
		t.Errorf("got %v does end with %v", got, want)
	}
}
