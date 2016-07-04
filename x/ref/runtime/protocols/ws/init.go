// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"v.io/v23/flow"

	websocket "v.io/x/ref/runtime/protocols/lib/websocket"
)

func init() {
	// ws, ws4, ws6 represent websocket protocol instances.
	protocol := websocket.WS{}
	flow.RegisterProtocol("ws", protocol, "ws4", "ws6")
	flow.RegisterProtocol("ws4", protocol)
	flow.RegisterProtocol("ws6", protocol)
}
