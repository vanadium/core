// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
)

var cmdInstantiate = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runInstantiate),
	Name:     "instantiate",
	Short:    "Create an instance of the given application.",
	Long:     "Create an instance of the given application, provide it with a blessing, and print the name of the new instance.",
	ArgsName: "<application installation> <grant extension>",
	ArgsLong: `
<application installation> is the vanadium object name of the
application installation from which to create an instance.

<grant extension> is used to extend the default blessing of the
current principal when blessing the app instance.`,
}

type granter struct {
	rpc.CallOpt
	extension string
}

func (g *granter) Grant(ctx *context.T, call security.Call) (security.Blessings, error) {
	p := call.LocalPrincipal()
	b, _ := p.BlessingStore().Default()
	return p.Bless(call.RemoteBlessings().PublicKey(), b, g.extension, security.UnconstrainedUse())
}

func runInstantiate(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("instantiate: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appInstallation, grant := args[0], args[1]

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	principal := v23.GetPrincipal(ctx)

	call, err := device.ApplicationClient(appInstallation).Instantiate(ctx)
	if err != nil {
		return fmt.Errorf("Instantiate failed: %v", err)
	}
	for call.RecvStream().Advance() {
		switch msg := call.RecvStream().Value().(type) {
		case device.BlessServerMessageInstancePublicKey:
			pubKey, err := security.UnmarshalPublicKey(msg.Value)
			if err != nil {
				return fmt.Errorf("Instantiate failed: %v", err)
			}
			// TODO(caprita,rthellend): Get rid of security.UnconstrainedUse().
			toextend, _ := principal.BlessingStore().Default()
			blessings, err := principal.Bless(pubKey, toextend, grant, security.UnconstrainedUse())
			if err != nil {
				return fmt.Errorf("Instantiate failed: %v", err)
			}
			call.SendStream().Send(device.BlessClientMessageAppBlessings{Value: blessings})
		default:
			fmt.Fprintf(env.Stderr, "Received unexpected message: %#v\n", msg)
		}
	}
	var instanceID string
	if instanceID, err = call.Finish(); err != nil {
		return fmt.Errorf("Instantiate failed: %v", err)
	}
	fmt.Fprintf(env.Stdout, "%s\n", naming.Join(appInstallation, instanceID))
	return nil
}
