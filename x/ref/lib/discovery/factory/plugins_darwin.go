// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

package factory

import (
	"v.io/x/ref/lib/discovery/plugins/mdns"
)

func init() {
	pluginFactories = pluginFactoryMap{
		"mdns": mdns.New,
	}
}
