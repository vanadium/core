// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"net"

	"v.io/v23/logging"
	"v.io/v23/rpc"
)

type addressChooser struct {
	logger logging.Logger
}

func (c *addressChooser) ChooseAddresses(protocol string, candidates []net.Addr) ([]net.Addr, error) {
	if ipaddr := CloudVMPublicAddress(); ipaddr != nil {
		c.logger.Infof("CloudVM public IP address: %v", ipaddr)
		return []net.Addr{ipaddr}, nil
	}
	return candidates, nil
}

// NewAddressChooser prefers the public IP if a process is running on a cloud
// service provider (such as Google Compute Engine or Amazon EC2), otherwise
// it keeps all the addresses of all interfaces.
func NewAddressChooser(logger logging.Logger) rpc.AddressChooser {
	return &addressChooser{logger: logger}
}
