// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ble

import (
	idiscovery "v.io/x/ref/lib/discovery"
)

const (
	// The Vanadium service base uuid and mask.
	vanadiumUUIDBase = "3dd1d5a8-0000-5000-0000-000000000000"
	vanadiumUUIDMask = "ffffffff-0000-f000-0000-000000000000"
)

var (
	vanadiumUUIDPrefixes = [...]byte{0x3d, 0xd1, 0xd5, 0xa8}
)

func newServiceUUID(interfaceName string) idiscovery.Uuid {
	uuid := idiscovery.NewServiceUUID(interfaceName)
	for i, prefix := range vanadiumUUIDPrefixes {
		uuid[i] = prefix
	}
	return uuid
}

func toggleServiceUUID(uuid idiscovery.Uuid) {
	uuid[len(uuid)-1] ^= 0x01
}
