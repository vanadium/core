// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"encoding/base64"
	"io/ioutil"
	"os"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline"
	lsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/services/agent/internal/ipc"
	"v.io/x/ref/services/agent/internal/server"
	"v.io/x/ref/services/cluster"

	_ "v.io/x/ref/runtime/factories/roaming"
)

var (
	clusterAgent  string
	socketPath    string
	secretKeyFile string
	rootBlessings string
)

func main() {
	cmdPodAgentD.Flags.StringVar(&clusterAgent, "agent", "", "The address of the cluster agent.")
	cmdPodAgentD.Flags.StringVar(&socketPath, "socket-path", "", "The path of the unix socket to listen on.")
	cmdPodAgentD.Flags.StringVar(&secretKeyFile, "secret-key-file", "", "The name of the file that contains the secret key.")
	cmdPodAgentD.Flags.StringVar(&rootBlessings, "root-blessings", "", "A comma-separated list of the root blessings to trust, base64-encoded VOM-encoded.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdPodAgentD)
}

var cmdPodAgentD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runPodAgentD),
	Name:   "pod_agentd",
	Short:  "Holds the principal of a kubernetes pod",
	Long: `
Command pod_agentd runs a security agent daemon, which holds a private key in
memory and makes it available to the kubernetes pod in which it is running.
`,
}

func runPodAgentD(ctx *context.T, env *cmdline.Env, args []string) error {
	p, err := lsecurity.NewPrincipal()
	if err != nil {
		return err
	}
	if ctx, err = v23.WithPrincipal(ctx, p); err != nil {
		return err
	}
	if rootBlessings != "" {
		addRoot(ctx, rootBlessings)
	}

	secret, err := ioutil.ReadFile(secretKeyFile)
	if err != nil {
		return err
	}

	// Fetch blessings from cluster agent.
	ca := cluster.ClusterAgentClient(clusterAgent)
	blessings, err := ca.SeekBlessings(ctx, string(secret))
	if err != nil {
		return err
	}
	if err = p.BlessingStore().SetDefault(blessings); err != nil {
		return err
	}
	if _, err = p.BlessingStore().Set(blessings, security.AllPrincipals); err != nil {
		return err
	}
	if err = security.AddToRoots(p, blessings); err != nil {
		return err
	}

	// Run the server.
	i := ipc.NewIPC()
	defer i.Close()
	if err = server.ServeAgent(i, lsecurity.MustForkPrincipal(
		p,
		lsecurity.ImmutableBlessingStore(p.BlessingStore()),
		lsecurity.ImmutableBlessingRoots(p.Roots()))); err != nil {
		return err
	}
	if _, err := os.Stat(socketPath); err == nil {
		os.Remove(socketPath)
	}
	if err = i.Listen(socketPath); err != nil {
		return err
	}
	// Make the socket available to all users so that the application can
	// run with a non-root UID.
	// The socket's parent directory is mounted only in the containers that
	// should have access to it. So, this doesn't change who has access to
	// the socket.
	if err = os.Chmod(socketPath, 0666); err != nil {
		return err
	}
	<-signals.ShutdownOnSignals(ctx)
	return nil
}

func addRoot(ctx *context.T, flagRoots string) {
	p := v23.GetPrincipal(ctx)
	for _, b64 := range strings.Split(flagRoots, ",") {
		// We use URLEncoding to be compatible with the principal
		// command.
		vomBlessings, err := base64.URLEncoding.DecodeString(b64)
		if err != nil {
			ctx.Fatalf("unable to decode the base64 blessing roots: %v", err)
		}
		var blessings security.Blessings
		if err := vom.Decode(vomBlessings, &blessings); err != nil {
			ctx.Fatalf("unable to decode the vom blessing roots: %v", err)
		}
		if err := security.AddToRoots(p, blessings); err != nil {
			ctx.Fatalf("unable to add blessing roots: %v", err)
		}
	}
}
