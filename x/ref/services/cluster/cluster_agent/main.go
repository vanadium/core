// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go .

package main

import (
	"encoding/base64"
	"fmt"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/cluster"
)

var (
	flagClusterAgentAddr string
	flagBase64           bool
)

var cmdNewSecret = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runNewSecret),
	Name:     "new",
	Short:    "Requests a new secret.",
	Long:     "Requests a new secret.",
	ArgsName: "<extension>",
	ArgsLong: `
<extension> is the blessing name extension to associate with the new secret.
`,
}

func runNewSecret(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); got != expected {
		return env.UsageErrorf("new: incorrect number of arguments, got %d, expected %d", got, expected)
	}
	extension := args[0]

	secret, err := cluster.ClusterAgentAdminClient(flagClusterAgentAddr).NewSecret(ctx, &granter{extension: extension})
	if err != nil {
		return err
	}
	if flagBase64 {
		// We use StdEncoding to be compatible with the kubernetes API.
		fmt.Fprintln(env.Stdout, base64.StdEncoding.EncodeToString([]byte(secret)))
	} else {
		fmt.Fprintln(env.Stdout, secret)
	}
	return nil
}

type granter struct {
	rpc.CallOpt
	extension string
}

func (g *granter) Grant(ctx *context.T, call security.Call) (security.Blessings, error) {
	p := call.LocalPrincipal()
	def, _ := p.BlessingStore().Default()
	return p.Bless(call.RemoteBlessings().PublicKey(), def, g.extension, security.UnconstrainedUse())
}

var cmdForgetSecret = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runForgetSecret),
	Name:     "forget",
	Short:    "Forgets an existing secret and its associated blessings.",
	Long:     "Forgets an existing secret and its associated blessings.",
	ArgsName: "<secret>",
	ArgsLong: `
<secret> is the secret to forget.
`,
}

func runForgetSecret(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); got != expected {
		return env.UsageErrorf("forget: incorrect number of arguments, got %d, expected %d", got, expected)
	}
	secret := args[0]
	if err := cluster.ClusterAgentAdminClient(flagClusterAgentAddr).ForgetSecret(ctx, secret); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Done")
	return nil
}

var cmdSeekBlessings = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runSeekBlessings),
	Name:   "seekblessings",
	Short:  "Retrieves all the blessings associated with a particular secret.",
	Long: `
Retrieves all the blessings associated with a particular secret.

The output is base64-encoded-vom-encoded blessings that are compatible with the
"principal set" command.
`,
	ArgsName: "<secret>",
	ArgsLong: `
<secret> is the secret to use.
`,
}

func runSeekBlessings(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); got != expected {
		return env.UsageErrorf("seekblessings: incorrect number of arguments, got %d, expected %d", got, expected)
	}
	secret := args[0]
	blessings, err := cluster.ClusterAgentAdminClient(flagClusterAgentAddr).SeekBlessings(ctx, secret)
	if err != nil {
		return err
	}

	data, err := vom.Encode(blessings)
	if err != nil {
		return err
	}
	// We use UrlEncoding to be compatible with the principal command.
	str := base64.URLEncoding.EncodeToString(data)
	fmt.Fprintln(env.Stdout, str)
	return nil
}

func main() {
	cmdline.HideGlobalFlagsExcept()

	cmdClusterAgentClient := &cmdline.Command{
		Name:  "cluster_agent",
		Short: "supports interactions with a cluster agent",
		Long:  "Command cluster_agent supports interactions with a cluster agent.",
		Children: []*cmdline.Command{
			cmdNewSecret,
			cmdForgetSecret,
			cmdSeekBlessings,
		},
	}
	cmdClusterAgentClient.Flags.StringVar(&flagClusterAgentAddr, "agent", "", "The name or address of the cluster agent server.")
	cmdNewSecret.Flags.BoolVar(&flagBase64, "base64", false, "If true, the secret is base64-encoded")
	cmdline.Main(cmdClusterAgentClient)
}
