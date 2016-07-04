// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"errors"
	"fmt"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/security/securityflag"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/cluster"
	impl "v.io/x/ref/services/cluster/internal"
)

var (
	rootDir     string
	publishName string
)

type authorizer struct {
	auth security.Authorizer
}

func (a *authorizer) Authorize(ctx *context.T, call security.Call) error {
	if call.Method() == "SeekBlessings" {
		return nil
	}
	return a.auth.Authorize(ctx, call)
}

func runServer(ctx *context.T, _ *cmdline.Env, _ []string) error {
	if rootDir == "" {
		return errors.New("--root-dir is not set.")
	}

	auth := securityflag.NewAuthorizerOrDie()
	if auth == nil {
		auth = security.DefaultAuthorizer()
	}

	agent := impl.NewAgent(v23.GetPrincipal(ctx), impl.NewFileStorage(rootDir))
	service := cluster.ClusterAgentAdminServer(impl.NewService(agent))

	ctx, server, err := v23.WithNewServer(ctx, publishName, service, &authorizer{auth})
	if err != nil {
		return err
	}

	status := server.Status()
	ctx.Infof("Listening on: %v", status.Endpoints)
	if len(status.Endpoints) > 0 {
		fmt.Printf("NAME=%s\n", status.Endpoints[0].Name())
	}
	<-signals.ShutdownOnSignals(ctx)
	return nil
}

func main() {
	rootCmd := &cmdline.Command{
		Name:  "cluster_agentd",
		Short: "Run cluster agent server",
		Long: `
The Cluster Agent keeps a list of Secret Keys and Blessings associated with
them. It issues new Blessings when presented with a valid Secret Key. The new
Blessings are extensions of the Blessings associated with the Secret Key.
`,
		Runner: v23cmd.RunnerFunc(runServer),
	}
	rootCmd.Flags.StringVar(&rootDir, "root-dir", "", "The directory where blessings are stored.")
	rootCmd.Flags.StringVar(&publishName, "name", "", "The vanadium object name under which to publish the cluster agent.")
	cmdline.Main(rootCmd)
}
