// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/mounttable"
	vdltime "v.io/v23/vdlroot/time"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

var now = time.Now()

func deadline(minutes int) vdltime.Deadline {
	return vdltime.Deadline{
		Time: now.Add(time.Minute * time.Duration(minutes)),
	}
}

type server struct {
	suffix string
}

//nolint:golint // API change required.
func (s *server) Glob__(ctx *context.T, call rpc.GlobServerCall, g *glob.Glob) error {
	ctx.VI(2).Infof("Glob() was called. suffix=%v pattern=%q", s.suffix, g.String())
	sender := call.SendStream()
	//nolint:errcheck
	sender.Send(naming.GlobReplyEntry{
		Value: naming.MountEntry{
			Name: "name1",
			Servers: []naming.MountedServer{
				{Server: "server1", Deadline: deadline(1)}},
			ServesMountTable: false,
			IsLeaf:           false,
		},
	})
	//nolint:errcheck
	sender.Send(naming.GlobReplyEntry{
		Value: naming.MountEntry{
			Name: "name2",
			Servers: []naming.MountedServer{
				{Server: "server2", Deadline: deadline(2)},
				{Server: "server3", Deadline: deadline(3)}},
			ServesMountTable: false,
			IsLeaf:           false,
		},
	})
	return nil
}

func (s *server) Mount(ctx *context.T, _ rpc.ServerCall, server string, ttl uint32, flags naming.MountFlag) error {
	ctx.VI(2).Infof("Mount() was called. suffix=%v server=%q ttl=%d", s.suffix, server, ttl)
	return nil
}

func (s *server) Unmount(ctx *context.T, _ rpc.ServerCall, server string) error {
	ctx.VI(2).Infof("Unmount() was called. suffix=%v server=%q", s.suffix, server)
	return nil
}

func (s *server) ResolveStep(ctx *context.T, _ rpc.ServerCall) (entry naming.MountEntry, err error) {
	ctx.VI(2).Infof("ResolveStep() was called. suffix=%v", s.suffix)
	entry.Servers = []naming.MountedServer{{Server: "server1", Deadline: deadline(1)}}
	entry.Name = s.suffix
	return
}

func (s *server) Delete(ctx *context.T, _ rpc.ServerCall, _ bool) error {
	ctx.VI(2).Infof("Delete() was called. suffix=%v", s.suffix)
	return nil
}
func (s *server) SetPermissions(ctx *context.T, _ rpc.ServerCall, _ access.Permissions, _ string) error {
	ctx.VI(2).Infof("SetPermissions() was called. suffix=%v", s.suffix)
	return nil
}

func (s *server) GetPermissions(ctx *context.T, _ rpc.ServerCall) (access.Permissions, string, error) {
	ctx.VI(2).Infof("GetPermissions() was called. suffix=%v", s.suffix)
	return nil, "", nil
}

type dispatcher struct {
}

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return mounttable.MountTableServer(&server{suffix: suffix}), nil, nil
}

func TestMountTableClient(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	_, server, err := v23.WithNewDispatchingServer(ctx, "", new(dispatcher))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	endpoint := server.Status().Endpoints[0]

	// Make sure to use our newly created mounttable rather than the
	// default.
	err = v23.GetNamespace(ctx).SetRoots(endpoint.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Setup the command-line.
	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}

	// Test the 'glob' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"glob", naming.JoinAddressName(endpoint.String(), ""), "*"}); err != nil {
		t.Fatalf("%v", err)
	}
	const deadRE = `\(Deadline ([^)]+)\)`
	if got, wantRE := strings.TrimSpace(stdout.String()), regexp.MustCompile("name1 server1 "+deadRE+"\nname2 server2 "+deadRE+" server3 "+deadRE); !wantRE.MatchString(got) {
		t.Errorf("got %q, want regexp %q", got, wantRE)
	}
	stdout.Reset()

	// Test the 'mount' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"mount", "server", endpoint.Name(), "123s"}); err != nil {
		t.Fatalf("%v", err)
	}
	if got, want := strings.TrimSpace(stdout.String()), "Name mounted successfully."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	stdout.Reset()

	// Test the 'unmount' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"unmount", "server", endpoint.Name()}); err != nil {
		t.Fatalf("%v", err)
	}
	if got, want := strings.TrimSpace(stdout.String()), "Unmount successful or name not mounted."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	stdout.Reset()

	// Test the 'resolvestep' command.
	ctx.Infof("resovestep %s", naming.JoinAddressName(endpoint.String(), "name"))
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"resolvestep", naming.JoinAddressName(endpoint.String(), "name")}); err != nil {
		t.Fatalf("%v", err)
	}
	if got, wantRE := strings.TrimSpace(stdout.String()), regexp.MustCompile(`Servers: \[\{server1 [^}]+\}\] Suffix: "name" MT: false`); !wantRE.MatchString(got) {
		t.Errorf("got %q, want regexp %q", got, wantRE)
	}
	stdout.Reset()
}
