// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

// Package roaming implements a RuntimeFactory suitable for a variety of network
// configurations, including 1-1 NATs, dhcp auto-configuration, Amazon Web
// Services and Google Compute Engine. It will adapt to networking changes
// such as the host obtaining a new IP address. This 'roaming' behaviour
// is disabled if cross compilation is used since it relies on cgo to
// access OS specific functionality for detecting and reading network
// configuration changes.
//
// The pubsub.Publisher mechanism is used for communicating networking
// settings to the rpc.Server implementation of the runtime and publishes
// the Settings it expects.
package roaming

import (
	"v.io/x/ref/runtime/factories/library"

	"v.io/v23/flow"

	"v.io/x/ref/runtime/protocols/lib/websocket"
	_ "v.io/x/ref/runtime/protocols/tcp" // Initialize tcp, ws and wsh.
	_ "v.io/x/ref/runtime/protocols/ws"
	_ "v.io/x/ref/runtime/protocols/wsh"
)

func init() {
	library.CloudVM = true
	library.ConfigureLoggingFromFlags = true
	library.ConfigurePermissionsFromFlags = true
	library.ReservedNameDispatcher = true
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
	library.EnableCommandlineFlags()
}
