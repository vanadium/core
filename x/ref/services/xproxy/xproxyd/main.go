// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"

	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/internal/statslib"
	"v.io/x/ref/services/xproxy/xproxy"
)

var healthzAddr, name, acl, statsACL string

const healthTimeout = 10 * time.Second

func main() {
	cmdProxyD.Flags.StringVar(&healthzAddr, "healthz-address", "", "Network address on which the HTTP healthz server runs.  It is intended to be used with a load balancer.  The load balancer must be able to reach this address in order to verify that the proxy server is running.")
	cmdProxyD.Flags.StringVar(&name, "name", "", "Name to mount the proxy as.")
	cmdProxyD.Flags.StringVar(&acl, "access-list", "", "Blessings that are authorized to listen via the proxy.  JSON-encoded representation of access.AccessList.  An empty string implies the default authorization policy.")
	cmdProxyD.Flags.StringVar(&statsACL, "stats-access-list", "", "Blessings that are authorized to access the proxy's statistics.  JSON-encoded representation of access.AccessList.  An empty string implies the default authorization policy.")

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
	ctx, handler := signals.ShutdownOnSignalsWithCancel(ctx)
	defer handler.WaitForSignal()
	// TODO(suharshs): Add ability to specify multiple proxies through this tool.
	auth, err := authorizer(ctx, acl)
	if err != nil {
		return fmt.Errorf("failed to parse --access-list: %v", err)
	}
	if auth != nil {
		ctx.Infof("Using access list to control proxy use: %v", auth.(access.AccessList))
	} else {
		ctx.Infof("--access-list not specified")
	}

	statsAuth, err := authorizer(ctx, statsACL)
	if err != nil {
		return fmt.Errorf("failed to parse --stats-access-list: %v", err)
	}
	if statsAuth != nil {
		ctx.Infof("Using access list to control access to statistics: %v", statsAuth.(access.AccessList))
	} else {
		ctx.Infof("--stats-access-list not specified")
	}
	proxyCtx, proxyCancel := context.WithCancel(ctx)
	proxy, err := xproxy.New(proxyCtx, name, auth)
	if err != nil {
		return err
	}
	handler.RegisterCancel(func() {
		proxyCancel()
		<-proxy.Closed()
	})
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

	var monitoringName string
	if len(name) > 0 {
		monitoringName = name + "-mon"
	}

	// Start a Stats service that is as accessible as the proxy itself but
	// with it's own ACL.
	_, statsServer, err := v23.WithNewDispatchingServer(
		ctx,
		monitoringName,
		&statsDispatcher{statsAuth})
	if err != nil {
		return fmt.Errorf("NewServer failed: %v", err)
	}

	if len(name) > 0 {
		eps := statsServer.Status().Endpoints
		if len(eps) == 1 {
			// Print out the address of the stats server for integration tests.
			fmt.Printf("STATS=%s\n", eps[0].Name())
		}
	}
	fmt.Printf("Proxy stats listening on: %v\n", statsServer.Status().Endpoints)
	return nil
}

type statsDispatcher struct {
	auth security.Authorizer
}

func (d *statsDispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return statslib.NewStatsService(suffix, 100*time.Millisecond), d.auth, nil
}

// healthzHandler implements net/http.Handler
type healthzHandler struct{}

func (healthzHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("ok")) //nolint:errcheck
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

func authorizer(ctx *context.T, acl string) (security.Authorizer, error) {
	if len(acl) > 0 {
		var list access.AccessList
		if err := json.NewDecoder(bytes.NewBufferString(acl)).Decode(&list); err != nil {
			return nil, err
		}
		// Always add ourselves, for the the reserved methods server
		// started below.
		list.In = append(list.In, security.DefaultBlessingPatterns(v23.GetPrincipal(ctx))...)
		return list, nil
	}
	return nil, nil
}
