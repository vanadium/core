// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"fmt"

	"v.io/v23/rpc"
)

// This file contains code in the impl package that we only want built for tests
// (it exposes public API methods that we don't want to normally expose).

func (c *callbackState) leaking() bool {
	c.Lock()
	defer c.Unlock()
	return len(c.channels) > 0
}

func DispatcherLeaking(d rpc.Dispatcher) bool {
	switch obj := d.(type) {
	case *dispatcher:
		return obj.internal.callback.leaking()
	case *testModeDispatcher:
		return obj.realDispatcher.(*dispatcher).internal.callback.leaking()
	default:
		panic(fmt.Sprintf("unexpected type: %T", d))
	}
}
