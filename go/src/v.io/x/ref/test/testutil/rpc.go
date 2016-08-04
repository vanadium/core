// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"v.io/v23/rpc"
)

// WaitForServerPublished blocks until all published mounts/unmounts have reached
// their desired state, and returns the resulting server status.
func WaitForServerPublished(s rpc.Server) rpc.ServerStatus {
	for {
		status := s.Status()
		if checkAllPublished(status) {
			return status
		}
		<-status.Dirty
	}
}

// WaitForProxyEndpoints blocks until the server's proxied endpoints appear in
// status.Endpoints, and returns the resulting server status.
func WaitForProxyEndpoints(s rpc.Server, proxyName string) rpc.ServerStatus {
	for {
		status := s.Status()
		if err, ok := status.ProxyErrors[proxyName]; ok && err == nil {
			return status
		}
		<-status.Dirty
	}
}

func checkAllPublished(status rpc.ServerStatus) bool {
	for _, e := range status.PublisherStatus {
		if e.LastState != e.DesiredState {
			return false
		}
	}
	return true
}
