// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"bytes"
	"fmt"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/examples/tunnel"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
)

var name, aclLiteral, aclFile string

func main() {
	cmdRoot.Flags.StringVar(&name, "name", "", "Name to publish the server as.")
	cmdRoot.Flags.StringVar(&aclFile, "acl-file", "", "File containing JSON-encoded Permissions.")
	cmdRoot.Flags.StringVar(&aclLiteral, "acl", "", "JSON-encoded Permissions.  Takes precedence over --acl-file.")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

var cmdRoot = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runTunnelD),
	Name:   "tunneld",
	Short:  "Runs the tunneld daemon",
	Long: `
Command tunneld runs the tunneld daemon, which implements the Tunnel interface.
`,
}

func runTunnelD(ctx *context.T, env *cmdline.Env, args []string) error {
	var (
		auth security.Authorizer
		err  error
	)
	ctx, handler := signals.ShutdownOnSignalsWithCancel(ctx)
	defer handler.WaitForSignal()
	switch {
	case aclLiteral != "":
		var perms access.Permissions
		if perms, err = access.ReadPermissions(bytes.NewBufferString(aclLiteral)); err != nil {
			return fmt.Errorf("ReadPermissions(%v) failed: %v", aclLiteral, err)
		}
		auth = access.TypicalTagTypePermissionsAuthorizer(perms)
	case aclFile != "":
		if auth, err = access.PermissionsAuthorizerFromFile(aclFile, access.TypicalTagType()); err != nil {
			return fmt.Errorf("PermissionsAuthorizerFromFile(%v) failed: %v", aclFile, err)
		}
	}
	ctx, server, err := v23.WithNewServer(ctx, name, tunnel.TunnelServer(&T{}), auth)
	if err != nil {
		return fmt.Errorf("NewServer failed: %v", err)
	}
	status := server.Status()
	ctx.Infof("Listening on: %v", status.Endpoints)
	if len(status.Endpoints) > 0 {
		fmt.Printf("NAME=%s\n", status.Endpoints[0].Name())
	}
	ctx.Infof("Published as %q", name)

	return nil
}
