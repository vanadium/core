// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !linux android

package internal

import (
	"net"
)

// InitCloudVM is a no-op on this platform.
func InitCloudVM() (func(), error) {
	return func() {}, nil
}

// CloudVMPublicAddress returns the public IP address of the Cloud VM instance
// it is run from, or nil if run from anywhere else. The returned address is the
// public address of a 1:1 NAT tunnel to this host.
func CloudVMPublicAddress() *net.IPAddr {
	return nil
}
