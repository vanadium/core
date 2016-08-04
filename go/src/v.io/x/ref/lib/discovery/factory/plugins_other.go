// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !android,!darwin

package factory

import "v.io/x/ref/lib/discovery/plugins/mdns"

func init() {
	pluginFactories = pluginFactoryMap{
		"mdns": mdns.New,

		// TODO(jhahn): The paypal gatt library, which the ble plugin currently
		// depends on does not work in some OS like iOS. Also it does not check
		// whether the device is available. Disable it by default for now.
		//
		// "ble":  ble.New,
	}
}
