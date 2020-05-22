// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pproflib_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/internal/pproflib"
	"v.io/x/ref/test"
)

type dispatcher struct {
	server interface{}
}

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return d.server, nil, nil
}

func TestPProfProxy(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	ctx, s, err := v23.WithNewDispatchingServer(ctx, "", &dispatcher{pproflib.NewPProfService()})
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	endpoints := s.Status().Endpoints
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	//nolint:errcheck
	go http.Serve(ln, pproflib.PprofProxy(ctx, "/myprefix", endpoints[0].Name()))
	testcases := []string{
		"/myprefix/pprof/",
		"/myprefix/pprof/cmdline",
		"/myprefix/pprof/profile?seconds=1",
		"/myprefix/pprof/heap",
		"/myprefix/pprof/goroutine",
		fmt.Sprintf("/pprof/symbol?%p", TestPProfProxy),
	}
	for _, c := range testcases {
		url := fmt.Sprintf("http://%s%s", ln.Addr(), c)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("%v: http.Get failed: %v", url, err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("%v: unexpected status code. Got %d, want 200", url, resp.StatusCode)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("%v: ReadAll failed: %v", url, err)
			continue
		}
		resp.Body.Close()
		if len(body) == 0 {
			t.Errorf("%v: unexpected empty body", url)
		}
	}
}
