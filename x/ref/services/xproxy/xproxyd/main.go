// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"

	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/xproxy/xproxy"
)

var healthzAddr, name, acl string

const healthTimeout = 10 * time.Second

func main() {
	cmdProxyD.Flags.StringVar(&healthzAddr, "healthz-address", "", "Network address on which the HTTP healthz server runs.  It is intended to be used with a load balancer.  The load balancer must be able to reach this address in order to verify that the proxy server is running.")
	cmdProxyD.Flags.StringVar(&name, "name", "", "Name to mount the proxy as.")
	cmdProxyD.Flags.StringVar(&acl, "access-list", "", "Blessings that are authorized to listen via the proxy.  JSON-encoded representation of access.AccessList.  An empty string implies the default authorization policy.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdProxyD)
}

var cmdProxyD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runProxyD),
	Name:   "xproxyd",
	Short:  "Proxies services to the outside world",
	Long: `
Command proxyd is a daemon that listens for connections from Vanadium services
(typically behind NATs) and proxies these services to the outside world.
`,
}

func runProxyD(ctx *context.T, env *cmdline.Env, args []string) error {
	// TODO(suharshs): Add ability to specify multiple proxies through this tool.
	auth, err := authorizer(ctx)
	if err != nil {
		return err
	}
	proxy, err := xproxy.New(ctx, name, auth)
	if err != nil {
		return err
	}
	peps := proxy.ListeningEndpoints()
	proxyEndpoint := peps[0]

	if len(name) > 0 {
		// Print out a directly accessible name for the proxy table so
		// that integration tests can reliably read it from stdout.
		fmt.Printf("NAME=%s\n", proxyEndpoint.Name())
	} else {
		fmt.Printf("Proxy listening on %s\n", proxyEndpoint)
	}

	if len(healthzAddr) != 0 {
		go startHealthzServer(ctx, healthzAddr)
	}

	// Start an RPC Server that listens through the proxy itself. This
	// server will serve reserved methods only.
	var monitoringName string
	if len(name) > 0 {
		monitoringName = name + "-mon"
	}
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Proxy: proxyEndpoint.Name()})
	if _, _, err := v23.WithNewDispatchingServer(ctx, monitoringName, &nilDispatcher{}); err != nil {
		return fmt.Errorf("NewServer failed: %v", err)
	}
	<-signals.ShutdownOnSignals(ctx)
	return nil
}

type nilDispatcher struct{}

func (nilDispatcher) Lookup(*context.T, string) (interface{}, security.Authorizer, error) {
	return nil, nil, nil
}

// healthzHandler implements net/http.Handler
type healthzHandler struct{}

func (healthzHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("ok"))
}

// startHealthzServer starts a HTTP server that simply returns "ok" to every
// request. This is needed to let the load balancer know that the proxy server
// is running.
func startHealthzServer(ctx *context.T, addr string) {
	s := http.Server{
		Addr:         addr,
		Handler:      healthzHandler{},
		ReadTimeout:  healthTimeout,
		WriteTimeout: healthTimeout,
	}
	if err := s.ListenAndServe(); err != nil {
		ctx.Fatal(err)
	}
}

func authorizer(ctx *context.T) (security.Authorizer, error) {
	if len(acl) > 0 {
		var list access.AccessList
		if err := json.NewDecoder(bytes.NewBufferString(acl)).Decode(&list); err != nil {
			return nil, err
		}
		// Always add ourselves, for the the reserved methods server
		// started below.
		list.In = append(list.In, security.DefaultBlessingPatterns(v23.GetPrincipal(ctx))...)
		ctx.Infof("Using access list to control proxy use: %v", list)
		return list, nil
	}
	return nil, nil
}
