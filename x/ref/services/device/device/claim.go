// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/base64"
	"fmt"

	"v.io/v23/context"
	"v.io/v23/options"
	"v.io/v23/security"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
)

var cmdClaim = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runClaim),
	Name:     "claim",
	Short:    "Claim the device.",
	Long:     "Claim the device.",
	ArgsName: "<device> <grant extension> <pairing token> <device publickey>",
	ArgsLong: `
<device> is the vanadium object name of the device manager's device service.

<grant extension> is used to extend the default blessing of the
current principal when blessing the app instance.

<pairing token> is a token that the device manager expects to be replayed
during a claim operation on the device.

<device publickey> is the marshalled public key of the device manager we
are claiming.`,
}

func runClaim(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, max, got := 2, 4, len(args); expected > got || got > max {
		return env.UsageErrorf("claim: incorrect number of arguments, expected atleast %d (max: %d), got %d", expected, max, got)
	}
	deviceName, grant := args[0], args[1]
	var pairingToken string
	if len(args) > 2 {
		pairingToken = args[2]
	}
	var serverAuth security.Authorizer
	if len(args) > 3 {
		marshalledPublicKey, err := base64.URLEncoding.DecodeString(args[3])
		if err != nil {
			return fmt.Errorf("Failed to base64 decode publickey: %v", err)
		}
		if deviceKey, err := security.UnmarshalPublicKey(marshalledPublicKey); err != nil {
			return fmt.Errorf("Failed to unmarshal device public key:%v", err)
		} else {
			serverAuth = security.PublicKeyAuthorizer(deviceKey)
		}
	} else {
		// Skip server endpoint authorization since an unclaimed device might
		// have roots that will not be recognized by the claimer.
		serverAuth = security.AllowEveryone()
	}
	if err := device.ClaimableClient(deviceName).Claim(ctx,
		pairingToken,
		&granter{extension: grant},
		options.ServerAuthorizer{Authorizer: serverAuth},
		options.NameResolutionAuthorizer{Authorizer: security.AllowEveryone()}); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Successfully claimed.")
	return nil
}
