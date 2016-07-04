// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package version

import (
	"fmt"

	"v.io/v23/rpc/version"
	"v.io/x/lib/metadata"
)

// Supported represents the range of protocol verions supported by this
// implementation.
//
// Max is incremented whenever we make a protocol change that's not both forward
// and backward compatible.
//
// Min is incremented whenever we want to remove support for old protocol
// versions.
var Supported = version.RPCVersionRange{Min: version.RPCVersion10, Max: version.RPCVersion14}

func init() {
	metadata.Insert("v23.RPCVersionMax", fmt.Sprint(Supported.Max))
	metadata.Insert("v23.RPCVersionMin", fmt.Sprint(Supported.Min))
}
