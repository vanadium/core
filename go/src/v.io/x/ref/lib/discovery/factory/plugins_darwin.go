// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

package factory

import (
	"v.io/v23/context"
	"v.io/x/ref/lib/discovery/plugins/ble"
	"v.io/x/ref/lib/discovery/plugins/ble/corebluetooth"
	"v.io/x/ref/lib/discovery/plugins/mdns"
)

func init() {
	bleFactory := func(ctx *context.T, _ string) (ble.Driver, error) {
		return corebluetooth.New(ctx)
	}
	ble.SetDriverFactory(bleFactory)

	pluginFactories = pluginFactoryMap{
		"mdns": mdns.New,
		"ble":  ble.New,
	}
}
