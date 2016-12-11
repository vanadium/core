// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tcp

import (
	"v.io/v23/flow"
	"v.io/x/ref/runtime/protocols/lib/tcputil"
)

func init() {
	tcp := tcputil.TCP{}
	flow.RegisterProtocol("tcp", tcp, "tcp4", "tcp6")
	flow.RegisterProtocol("tcp4", tcp)
	flow.RegisterProtocol("tcp6", tcp)
}
