// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"fmt"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"

	_ "v.io/x/ref/runtime/factories/generic"
)

func main() {
	flags.SetDefaultHostPort("127.0.0.1:0")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdPingPong)
}

var cmdPingPong = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runPingPong),
	Name:   "pingpong",
	Short:  "Runs pingpong client or server",
	Long: `
Command pingpong runs a pingpong client or server.  If no args are given the
server is run, otherwise the client is run.
`,
	ArgsName: "[server]",
	ArgsLong: `
If [server] is specified, pingpong is run in client mode, and connects to the
named server.
`,
}

type pongd struct{}

func (f *pongd) Ping(ctx *context.T, call rpc.ServerCall, message string) (result string, err error) {
	client, _ := security.RemoteBlessingNames(ctx, call.Security())
	server := security.LocalBlessingNames(ctx, call.Security())
	return fmt.Sprintf("pong (client:%v server:%v)", client, server), nil
}

func clientMain(ctx *context.T, server string) error {
	fmt.Println("Pinging...")
	pong, err := PingPongClient(server).Ping(ctx, "ping")
	if err != nil {
		return fmt.Errorf("error pinging: %v", err)
	}
	fmt.Println(pong)
	return nil
}

func serverMain(ctx *context.T) error {
	// Provide an empty name, no need to mount on any mounttable.
	//
	// Use the default authorization policy (nil authorizer), which will
	// only authorize clients if the blessings of the client is a prefix of
	// that of the server or vice-versa.
	ctx, s, err := v23.WithNewServer(ctx, "", PingPongServer(&pongd{}), nil)
	if err != nil {
		return fmt.Errorf("failure creating server: %v", err)
	}
	ctx.Info("Waiting for ping")
	fmt.Printf("NAME=%v\n", s.Status().Endpoints[0].Name())
	// Wait forever.
	<-signals.ShutdownOnSignals(ctx)
	return nil
}

func runPingPong(ctx *context.T, env *cmdline.Env, args []string) error {
	switch len(args) {
	case 0:
		return serverMain(ctx)
	case 1:
		return clientMain(ctx, args[0])
	}
	return env.UsageErrorf("Too many arguments")
}
