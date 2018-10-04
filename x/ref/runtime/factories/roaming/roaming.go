// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

// Package roaming implements a RuntimeFactory suitable for a variety of network
// configurations, including 1-1 NATs, dhcp auto-configuration, Amazon Web
// Services and Google Compute Engine.
//
// The pubsub.Publisher mechanism is used for communicating networking
// settings to the rpc.Server implementation of the runtime and publishes
// the Settings it expects.
package roaming

import (
	"flag"

	"v.io/x/ref/runtime/factories/library"

	"v.io/v23/flow"

	"v.io/x/ref/lib/flags"
	"v.io/x/ref/runtime/protocols/lib/websocket"
	_ "v.io/x/ref/runtime/protocols/tcp" // Initialize tcp, ws and wsh.
	_ "v.io/x/ref/runtime/protocols/ws"
	_ "v.io/x/ref/runtime/protocols/wsh"
)

func init() {
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
	library.Roam = true
	library.CloudVM = true
	library.ConfigureLoggingFromFlags = true
	library.ReservedNameDispatcher = true
	flags.RegisterListenFlags(flag.CommandLine, &library.ListenFlags)
	flags.RegisterRuntimeFlags(flag.CommandLine, &library.RuntimeFlags)
	flags.RegisterPermissionsFlags(flag.CommandLine, &library.PermissionsFlags)
}
