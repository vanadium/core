// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

// Package static implements a RuntimeFactory suitable for a variety of network
// configurations, including 1-1 NATs, Amazon Web Services and Google Compute
// Engine but hosted on a static IP address with no support for adapting to
// dhcp changes.
package static

import (
	"v.io/x/ref/runtime/factories/library"

	"v.io/v23/flow"

	"v.io/x/ref/runtime/protocols/lib/websocket"
	_ "v.io/x/ref/runtime/protocols/tcp" // Initialize tcp, ws and wsh.
	_ "v.io/x/ref/runtime/protocols/ws"
	_ "v.io/x/ref/runtime/protocols/wsh"
)

func init() {
	library.Roam = false
	library.CloudVM = true
	library.ConfigureLoggingFromFlags = true
	library.ConfigurePermissionsFromFlags = true
	library.ReservedNameDispatcher = true
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
	library.EnableCommandlineFlags()
}
