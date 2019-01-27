// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package generic implements a RuntimeFactory that is useful in tests. It
// prefers listening on localhost addresses.
package generic

import (
	"v.io/v23/flow"

	"v.io/x/ref/lib/flags"
	"v.io/x/ref/runtime/factories/library"
	"v.io/x/ref/runtime/protocols/lib/websocket"
	_ "v.io/x/ref/runtime/protocols/tcp" // Initialize tcp, ws and wsh.
	_ "v.io/x/ref/runtime/protocols/ws"
	_ "v.io/x/ref/runtime/protocols/wsh"
)

func init() {
	library.Roam = false
	library.CloudVM = false
	library.ReservedNameDispatcher = false
	library.ConfigureLoggingFromFlags = true
	library.ConfigurePermissionsFromFlags = true
	library.AllowMultipleInitializations = true
	flow.RegisterUnknownProtocol("wsh", websocket.WSH{})
	flags.SetDefaultHostPort(":0")
	library.EnableCommandlineFlags()
}
