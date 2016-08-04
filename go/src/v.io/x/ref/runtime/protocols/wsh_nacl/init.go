// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build nacl

package wsh_nacl

import (
	"v.io/v23/flow"

	websocket "v.io/x/ref/runtime/protocols/lib/websocket"
)

func init() {
	// We limit wsh to ws since in general nacl does not allow direct access
	// to TCP/UDP networking.
	wshNaCl := websocket.WS{}
	flow.RegisterProtocol("wsh", wshNaCl, "ws4", "ws6")
	flow.RegisterProtocol("wsh4", wshNaCl, "ws4")
	flow.RegisterProtocol("wsh6", wshNaCl, "ws6")
}
