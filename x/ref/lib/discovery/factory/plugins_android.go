// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build android

package factory

import (
	"v.io/x/ref/lib/discovery/plugins/ble"
	"v.io/x/ref/lib/discovery/plugins/mdns"
)

func init() {
	pluginFactories = pluginFactoryMap{
		"mdns": mdns.New,
		"ble":  ble.New,
	}
}
