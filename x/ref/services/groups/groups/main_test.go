// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/v23/services/groups"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"
)

var group map[string]struct{}
var buffer bytes.Buffer

type mock struct{}

func (mock) Create(ctx *context.T, call rpc.ServerCall, perms access.Permissions, entries []groups.BlessingPatternChunk) error {
	fmt.Fprintf(&buffer, "Create(%v, %v) was called", perms, entries)
	group = make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		group[string(entry)] = struct{}{}
	}
	return nil
}

func (mock) Delete(ctx *context.T, call rpc.ServerCall, version string) error {
	fmt.Fprintf(&buffer, "Delete(%v) was called", version)
	group = nil
	return nil
}

func (mock) Add(ctx *context.T, call rpc.ServerCall, entry groups.BlessingPatternChunk, version string) error {
	fmt.Fprintf(&buffer, "Add(%v, %v) was called", entry, version)
	group[string(entry)] = struct{}{}
	return nil
}

func (mock) Remove(ctx *context.T, call rpc.ServerCall, entry groups.BlessingPatternChunk, version string) error {
	fmt.Fprintf(&buffer, "Remove(%v, %v) was called", entry, version)
	delete(group, string(entry))
	return nil
}

func (mock) Relate(ctx *context.T, call rpc.ServerCall, blessings map[string]struct{}, hint groups.ApproximationType, version string, visitedGroups map[string]struct{}) (map[string]struct{}, []groups.Approximation, string, error) {
	fmt.Fprintf(&buffer, "Relate(%v, %v, %v, %v) was called", blessings, hint, version, visitedGroups)
	return nil, nil, "123", nil
}

func (mock) Get(ctx *context.T, call rpc.ServerCall, req groups.GetRequest, version string) (groups.GetResponse, string, error) {
	return groups.GetResponse{}, "", nil
}

func (mock) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	return nil
}

func (mock) GetPermissions(ctx *context.T, call rpc.ServerCall) (access.Permissions, string, error) {
	return nil, "", nil
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	rune, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(rune)) + s[size:]
}

func startServer(ctx *context.T, t *testing.T) (rpc.Server, naming.Endpoint) {
	unpublished := ""
	_, s, err := v23.WithNewServer(ctx, unpublished, groups.GroupServer(&mock{}), nil)
	if err != nil {
		t.Fatalf("NewServer(%v) failed: %v", unpublished, err)
	}
	return s, s.Status().Endpoints[0]
}

func TestGroupClient(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23Init()
	defer shutdown()

	ctx, cancel := context.WithCancel(ctx)
	server, endpoint := startServer(ctx, t)
	defer func() {
		cancel()
		<-server.Closed()
	}()

	// Test the "create" command.
	{
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		patterns := []string{"alice", "bob"}
		args := append([]string{"create", naming.JoinAddressName(endpoint.String(), "")}, patterns...)
		if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, args); err != nil {
			t.Fatalf("run failed: %v\n%v", err, stderr.String())
		}
		if got, want := buffer.String(), fmt.Sprintf("Create(%v, %v) was called", access.Permissions{}, patterns); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := strings.TrimSpace(stdout.String()), ""; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if got, want := len(group), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		buffer.Reset()
	}

	// Test the "add" command.
	{
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		pattern, version := "charlie", "123"
		args := []string{"add", "-version=" + version, naming.JoinAddressName(endpoint.String(), ""), pattern}
		if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, args); err != nil {
			t.Fatalf("run failed: %v\n%v", err, stderr.String())
		}
		if got, want := buffer.String(), fmt.Sprintf("Add(%v, %v) was called", pattern, version); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := strings.TrimSpace(stdout.String()), ""; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if got, want := len(group), 3; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		buffer.Reset()
	}

	// Test the "remove" command.
	{
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		pattern, version := "bob", "123"
		args := []string{"remove", "-version=" + version, naming.JoinAddressName(endpoint.String(), ""), "bob"}
		if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, args); err != nil {
			t.Fatalf("run failed: %v\n%v", err, stderr.String())
		}
		if got, want := buffer.String(), fmt.Sprintf("Remove(%v, %v) was called", pattern, version); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := strings.TrimSpace(stdout.String()), ""; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if got, want := len(group), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		buffer.Reset()
	}

	// Test the "delete" command.
	{
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		version := "123"
		args := []string{"delete", "-version=" + version, naming.JoinAddressName(endpoint.String(), "")}
		if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, args); err != nil {
			t.Fatalf("run failed: %v\n%v", err, stderr.String())
		}
		if got, want := buffer.String(), fmt.Sprintf("Delete(%v) was called", version); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := strings.TrimSpace(stdout.String()), ""; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if got, want := len(group), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		buffer.Reset()
	}

	// Test the "relate" command.
	{
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		hint, version := "over", "123"
		args := []string{"relate", "-approximation=" + hint, "-version=" + version, naming.JoinAddressName(endpoint.String(), "")}
		if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, args); err != nil {
			t.Fatalf("run failed: %v\n%v", err, stderr.String())
		}
		empty := map[string]struct{}{}
		if got, want := buffer.String(), fmt.Sprintf("Relate(%v, %v, %v, %v) was called", empty, capitalize(hint), version, empty); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		var got relateResult
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatalf("Unmarshal(%v) failed: %v", stdout.Bytes(), err)
		}
		want := relateResult{
			Remainder:      nil,
			Approximations: nil,
			Version:        "123",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
		buffer.Reset()
	}
}
