// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"io"
	"reflect"
	"sort"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/x/ref/lib/stats"
	irpc "v.io/x/ref/runtime/internal/rpc"
	"v.io/x/ref/services/debug/debuglib"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func TestDebugServer(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Setup the client and server principals, with the client willing to share its
	// blessing with the server.
	var (
		pclient = testutil.NewPrincipal()
		cctx, _ = v23.WithPrincipal(ctx, pclient)
	)
	idp := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
	if err := idp.Bless(pclient, "client"); err != nil {
		t.Fatal(err)
	}
	name := "testserver"
	debugDisp := debuglib.NewDispatcher(nil)
	_, _, err := v23.WithNewServer(ctx, name, &testObject{}, nil,
		irpc.ReservedNameDispatcher{Dispatcher: debugDisp})
	if err != nil {
		t.Fatal(err)
	}

	// Call the Foo method on ""
	{
		var value string
		if err := v23.GetClient(cctx).Call(cctx, name, "Foo", nil, []interface{}{&value}); err != nil {
			t.Fatalf("client.Call failed: %v", err)
		}
		if want := "BAR"; value != want {
			t.Errorf("unexpected value: Got %v, want %v", value, want)
		}
	}
	// Call Value on __debug/stats/testing/foo
	{
		foo := stats.NewString("testing/foo")
		foo.Set("The quick brown fox jumps over the lazy dog")
		fullname := naming.Join(name, "__debug/stats/testing/foo")
		var value string
		if err := v23.GetClient(cctx).Call(cctx, fullname, "Value", nil, []interface{}{&value}); err != nil {
			t.Fatalf("client.Call failed: %v", err)
		}
		if want := foo.Value(); value != want {
			t.Errorf("unexpected result: Got %v, want %v", value, want)
		}
	}

	// Call Glob
	testcases := []struct {
		name, pattern string
		expected      []string
	}{
		{"", "*", []string{}},
		{"", "__*", []string{"__debug"}},
		{"", "__*/*", []string{"__debug/http", "__debug/logs", "__debug/pprof", "__debug/stats", "__debug/vtrace"}},
		{"__debug", "*", []string{"http", "logs", "pprof", "stats", "vtrace"}},
	}
	for _, tc := range testcases {
		fullname := naming.Join(name, tc.name)
		call, err := v23.GetClient(ctx).StartCall(cctx, fullname, rpc.GlobMethod, []interface{}{tc.pattern})
		if err != nil {
			t.Fatalf("client.StartCall failed for %q: %v", tc.name, err)
		}
		results := []string{}
		for {
			var gr naming.GlobReply
			if err := call.Recv(&gr); err != nil {
				if err != io.EOF {
					t.Fatalf("Recv failed for %q: %v. Results received thus far: %q", tc.name, err, results)
				}
				break
			}
			if v, ok := gr.(naming.GlobReplyEntry); ok {
				results = append(results, v.Value.Name)
			}
		}
		if err := call.Finish(); err != nil {
			t.Fatalf("call.Finish failed for %q: %v", tc.name, err)
		}
		sort.Strings(results)
		if !reflect.DeepEqual(tc.expected, results) {
			t.Errorf("unexpected results for %q. Got %v, want %v", tc.name, results, tc.expected)
		}
	}
}

type testObject struct {
}

func (o testObject) Foo(*context.T, rpc.ServerCall) (string, error) {
	return "BAR", nil
}
