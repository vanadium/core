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

// CloudVMAddresses returns the private and public IP addresses of the Cloud VM
// instance it is run from, or nil if run from anywhere else. The returned
// public address is the address of a 1:1 NAT tunnel to this host.
func CloudVMAddresses() []net.Addr {
	return nil
}
