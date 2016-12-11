// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package wsh registers the websocket 'hybrid' protocol.
// We prefer to use tcp whenever we can to avoid the overhead of websockets.
package wsh

import (
	"v.io/v23/flow"

	websocket "v.io/x/ref/runtime/protocols/lib/websocket"
)

func init() {
	protocol := websocket.WSH{}
	flow.RegisterProtocol("wsh", protocol, "tcp4", "tcp6", "ws4", "ws6")
	flow.RegisterProtocol("wsh4", protocol, "tcp4", "ws4")
	flow.RegisterProtocol("wsh6", protocol, "tcp6", "ws6")
}
