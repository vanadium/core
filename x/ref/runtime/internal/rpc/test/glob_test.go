// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/i18n"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/rpc/reserved"
	"v.io/v23/security"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func TestGlob(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	namespace := []string{
		"a/b/c1/d1",
		"a/b/c1/d2",
		"a/b/c2/d1",
		"a/b/c2/d2",
		"a/x/y/z",
		"leaf",
	}
	tree := newNode()
	for _, p := range namespace {
		tree.find(strings.Split(p, "/"), true)
	}

	_, server, err := v23.WithNewDispatchingServer(ctx, "", &disp{tree})
	if err != nil {
		t.Fatalf("failed to start debug server: %v", err)
	}
	ep := server.Status().Endpoints[0].String()

	var (
		noExist        = verror.ErrNoExist.Errorf(ctx, "does not exist")
		notImplemented = reserved.ErrorfGlobNotImplemented(ctx, "glob not implemented")
		maxRecursion   = reserved.ErrorfGlobMaxRecursionReached(ctx, "max recursion level reached")
	)

	testcases := []struct {
		name, pattern string
		expected      []string
		errors        []naming.GlobError
	}{
		{"", "...", []string{
			"",
			"a",
			"a/b",
			"a/b/c1",
			"a/b/c1/d1",
			"a/b/c1/d2",
			"a/b/c2",
			"a/b/c2/d1",
			"a/b/c2/d2",
			"a/x",
			"a/x/y",
			"a/x/y/z",
			"leaf",
		}, nil},
		{"a", "...", []string{
			"",
			"b",
			"b/c1",
			"b/c1/d1",
			"b/c1/d2",
			"b/c2",
			"b/c2/d1",
			"b/c2/d2",
			"x",
			"x/y",
			"x/y/z",
		}, nil},
		{"a/b", "...", []string{
			"",
			"c1",
			"c1/d1",
			"c1/d2",
			"c2",
			"c2/d1",
			"c2/d2",
		}, nil},
		{"a/b/c1", "...", []string{
			"",
			"d1",
			"d2",
		}, nil},
		{"a/b/c1/d1", "...", []string{
			"",
		}, nil},
		{"a/x", "...", []string{
			"",
			"y",
			"y/z",
		}, nil},
		{"a/x/y", "...", []string{
			"",
			"z",
		}, nil},
		{"a/x/y/z", "...", []string{
			"",
		}, nil},
		{"", "", []string{""}, nil},
		{"", "*", []string{"a", "leaf"}, nil},
		{"a", "", []string{""}, nil},
		{"a", "*", []string{"b", "x"}, nil},
		{"a/b", "", []string{""}, nil},
		{"a/b", "*", []string{"c1", "c2"}, nil},
		{"a/b/c1", "", []string{""}, nil},
		{"a/b/c1", "*", []string{"d1", "d2"}, nil},
		{"a/b/c1/d1", "*", []string{}, nil},
		{"a/b/c1/d1", "", []string{""}, nil},
		{"a/b/c2", "", []string{""}, nil},
		{"a/b/c2", "*", []string{"d1", "d2"}, nil},
		{"a/b/c2/d1", "*", []string{}, nil},
		{"a/b/c2/d1", "", []string{""}, nil},
		{"a", "*/c?", []string{"b/c1", "b/c2"}, nil},
		{"a", "*/*", []string{"b/c1", "b/c2", "x/y"}, nil},
		{"a", "*/*/*", []string{"b/c1/d1", "b/c1/d2", "b/c2/d1", "b/c2/d2", "x/y/z"}, nil},
		{"a/x", "*/*", []string{"y/z"}, nil},
		{"bad", "", []string{}, []naming.GlobError{{Name: "", Error: noExist}}},
		{"bad/foo", "", []string{}, []naming.GlobError{{Name: "", Error: noExist}}},
		{"a/bad", "", []string{}, []naming.GlobError{{Name: "", Error: noExist}}},
		{"a/b/bad", "", []string{}, []naming.GlobError{{Name: "", Error: noExist}}},
		{"a/b/c1/bad", "", []string{}, []naming.GlobError{{Name: "", Error: noExist}}},
		{"a/x/bad", "", []string{}, []naming.GlobError{{Name: "", Error: noExist}}},
		{"a/x/y/bad", "", []string{}, []naming.GlobError{{Name: "", Error: noExist}}},
		{"leaf", "", []string{""}, nil},
		{"leaf", "*", []string{}, []naming.GlobError{{Name: "", Error: notImplemented}}},
		{"leaf/foo", "", []string{}, []naming.GlobError{{Name: "", Error: noExist}}},
		// muah is an infinite space to test rescursion limit.
		{"muah", "*", []string{"ha"}, nil},
		{"muah", "*/*", []string{"ha/ha"}, nil},
		{"muah", "*/*/*/*/*/*/*/*/*/*/*/*", []string{"ha/ha/ha/ha/ha/ha/ha/ha/ha/ha/ha/ha"}, nil},
		{"muah", "...", []string{
			"",
			"ha",
			"ha/ha",
			"ha/ha/ha",
			"ha/ha/ha/ha",
			"ha/ha/ha/ha/ha",
			"ha/ha/ha/ha/ha/ha",
			"ha/ha/ha/ha/ha/ha/ha",
			"ha/ha/ha/ha/ha/ha/ha/ha",
			"ha/ha/ha/ha/ha/ha/ha/ha/ha",
			"ha/ha/ha/ha/ha/ha/ha/ha/ha/ha",
		}, []naming.GlobError{{Name: "ha/ha/ha/ha/ha/ha/ha/ha/ha/ha/ha", Error: maxRecursion}}},
	}
	for _, tc := range testcases {
		name := naming.JoinAddressName(ep, tc.name)
		results, globErrors, err := testutil.GlobName(ctx, name, tc.pattern)
		if err != nil {
			t.Errorf("unexpected Glob error for (%q, %q): %v", tc.name, tc.pattern, err)
			continue
		}
		if !reflect.DeepEqual(results, tc.expected) {
			t.Errorf("unexpected result for (%q, %q). Got %q, want %q", tc.name, tc.pattern, results, tc.expected)
		}
		if len(globErrors) != len(tc.errors) {
			t.Errorf("unexpected number of glob errors for (%q, %q): got %#v, expected %#v", tc.name, tc.pattern, globErrors, tc.errors)
		}
		for i, e := range globErrors {
			if i >= len(tc.errors) {
				t.Errorf("unexpected glob error for (%q, %q): %#v", tc.name, tc.pattern, e.Error)
				continue
			}
			if e.Name != tc.errors[i].Name {
				t.Errorf("unexpected glob error for (%q, %q): %v", tc.name, tc.pattern, e)
			}
			if got, expected := verror.ErrorID(e.Error), verror.ErrorID(tc.errors[i].Error); got != expected {
				t.Errorf("unexpected error ID for (%q, %q): Got %v, expected %v", tc.name, tc.pattern, got, expected)
			}
		}
	}
}

func TestGlobDeny(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	tree := newNode()
	tree.find([]string{"a", "b"}, true)
	tree.find([]string{"a", "deny", "x"}, true)

	_, server, err := v23.WithNewDispatchingServer(ctx, "", &disp{tree})
	if err != nil {
		t.Fatalf("failed to start debug server: %v", err)
	}
	ep := server.Status().Endpoints[0].String()

	testcases := []struct {
		name, pattern string
		results       []string
		numErrors     int
	}{
		{"", "*", []string{"a"}, 0},
		{"", "*/*", []string{"a/b"}, 1},
		{"a", "*", []string{"b"}, 1},
		{"a/deny", "*", []string{}, 1},
		{"a/deny/x", "*", []string{}, 1},
		{"a/deny/y", "*", []string{}, 1},
	}

	// Ensure that we're getting the english error message.
	ctx = i18n.WithLangID(ctx, i18n.LangID("en-US"))

	for _, tc := range testcases {
		name := naming.JoinAddressName(ep, tc.name)
		results, globErrors, err := testutil.GlobName(ctx, name, tc.pattern)
		if err != nil {
			t.Errorf("unexpected Glob error for (%q, %q): %v", tc.name, tc.pattern, err)
		}
		if !reflect.DeepEqual(results, tc.results) {
			t.Errorf("unexpected results for (%q, %q). Got %v, expected %v", tc.name, tc.pattern, results, tc.results)
		}
		if len(globErrors) != tc.numErrors {
			t.Errorf("unexpected number of errors for (%q, %q). Got %v, expected %v", tc.name, tc.pattern, len(globErrors), tc.numErrors)
		}
		for _, gerr := range globErrors {
			if got, expected := verror.ErrorID(gerr.Error), reserved.ErrGlobMatchesOmitted.ID; got != expected {
				t.Errorf("unexpected error for (%q, %q): Got %v, expected %v", tc.name, tc.pattern, got, expected)
			}
			// This error message is purposely vague to avoid leaking information that
			// the user doesn't have access to.
			// We check the actual error string to make sure that we don't start
			// leaking new information by accident.
			expectedStr := fmt.Sprintf(
				`test.test:"%s".__Glob: some matches might have been omitted`,
				tc.name)
			if got := gerr.Error.Error(); got != expectedStr {
				t.Errorf("unexpected error string: Got %q, expected %q", got, expectedStr)
			}
		}
	}
}

type disp struct {
	tree *node
}

func (d *disp) Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error) {
	elems := strings.Split(suffix, "/")
	var auth security.Authorizer
	for _, e := range elems {
		if e == "deny" {
			return &leafObject{}, &denyAllAuthorizer{}, nil
		}
	}
	if len(elems) != 0 && elems[0] == "muah" {
		// Infinite space. Each node has one child named "ha".
		return rpc.ChildrenGlobberInvoker("ha"), auth, nil

	}
	if len(elems) != 0 && elems[len(elems)-1] == "leaf" {
		return &leafObject{}, auth, nil
	}
	n := d.tree.find(elems, false)
	if n == nil {
		return nil, nil, verror.ErrNoExist.Errorf(ctx, "Does not exist: %v", suffix)
	}
	if len(elems) < 2 || (elems[0] == "a" && elems[1] == "x") {
		return &vChildrenObject{n}, auth, nil
	}
	return &globObject{n}, auth, nil
}

type denyAllAuthorizer struct{}

func (denyAllAuthorizer) Authorize(*context.T, security.Call) error {
	return errors.New("no access")
}

type globObject struct {
	n *node
}

//nolint:golint // API change required.
func (o *globObject) Glob__(ctx *context.T, call rpc.GlobServerCall, g *glob.Glob) error {
	o.globLoop(call, "", g, o.n)
	return nil
}

func (o *globObject) globLoop(call rpc.GlobServerCall, name string, g *glob.Glob, n *node) {
	if g.Len() == 0 {
		call.SendStream().Send(naming.GlobReplyEntry{Value: naming.MountEntry{Name: name}}) //nolint:errcheck
	}
	if g.Empty() {
		return
	}
	matcher, left := g.Head(), g.Tail()
	for leaf, child := range n.children {
		if matcher.Match(leaf) {
			o.globLoop(call, naming.Join(name, leaf), left, child)
		}
	}
}

type vChildrenObject struct {
	n *node
}

//nolint:golint // API change required.
func (o *vChildrenObject) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, m *glob.Element) error {
	sender := call.SendStream()
	for child := range o.n.children {
		if m.Match(child) {
			if err := sender.Send(naming.GlobChildrenReplyName{Value: child}); err != nil {
				return err
			}
		}
	}
	return nil
}

type node struct {
	children map[string]*node
}

func newNode() *node {
	return &node{make(map[string]*node)}
}

func (n *node) find(names []string, create bool) *node {
	if len(names) == 1 && names[0] == "" {
		return n
	}
	for {
		if len(names) == 0 {
			return n
		}
		if next, ok := n.children[names[0]]; ok {
			n = next
			names = names[1:]
			continue
		}
		if create {
			nn := newNode()
			n.children[names[0]] = nn
			n = nn
			names = names[1:]
			continue
		}
		return nil
	}
}

type leafObject struct{}

func (leafObject) Func(*context.T, rpc.ServerCall) error {
	return nil
}
