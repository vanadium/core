// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bridge

import (
	"fmt"

	"v.io/v23/rpc"
)

func MethodDesc(desc rpc.InterfaceDesc, name string) rpc.MethodDesc {
	for _, method := range desc.Methods {
		if method.Name == name {
			return method
		}
	}
	panic(fmt.Sprintf("unknown method: %s.%s", desc.Name, name))
}
