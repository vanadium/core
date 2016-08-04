// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux,!android

package internal

import (
	"net"
	"os"
	"sync"
	"time"

	"v.io/v23/verror"

	"v.io/x/ref"
	"v.io/x/ref/runtime/internal/cloudvm"
)

const pkgPath = "v.io/x/ref/runtime/internal"

var (
	errNotGoogleComputeEngine = verror.Register(pkgPath+".errNotGoogleComputeEngine", verror.NoRetry, "{1:}{2:} failed to access gce metadata")
	errGCENoExternalIP        = verror.Register(pkgPath+".errGCENoExternalIP", verror.NoRetry, "{1:}{2:} gce instance does not have an external IP address")

	initialized bool
	mu          sync.Mutex
)

// InitCloudVM initializes the CloudVM metadata.
//
// If EnvExpectGoogleComputeEngine is set, this function returns after the
// initialization is done. Otherwise, the initialization is done asynchronously
// and future calls to CloudVMPublicAddress() will block until the
// initialization is complete.
//
// Returns a function to cancel initialization.
func InitCloudVM() (func(), error) {
	mu.Lock()
	if initialized {
		mu.Unlock()
		return func() {}, nil
	}
	initialized = true
	cancel := make(chan struct{})
	if os.Getenv(ref.EnvExpectGoogleComputeEngine) != "" {
		defer mu.Unlock()
		cloudvm.InitGCE(30*time.Second, cancel)
		if !cloudvm.RunningOnGCE() {
			return func() {}, verror.New(errNotGoogleComputeEngine, nil)
		}
		if cloudvm.ExternalIPAddress() == nil {
			return func() {}, verror.New(errGCENoExternalIP, nil)
		}
		return func() {}, nil
	}
	done := make(chan struct{})
	go func() {
		cloudvm.InitGCE(time.Second, cancel)
		cloudvm.InitAWS(time.Second, cancel)
		mu.Unlock()
		close(done)
	}()
	return func() {
		close(cancel)
		<-done
	}, nil
}

// CloudVMPublicAddress returns the public IP address of the Cloud VM instance
// it is run from, or nil if run from anywhere else. The returned address is the
// public address of a 1:1 NAT tunnel to this host.
func CloudVMPublicAddress() *net.IPAddr {
	mu.Lock()
	defer mu.Unlock()
	if !initialized {
		panic("CloudVMPublicAddress called before InitCloudVM")
	}
	if !cloudvm.RunningOnGCE() && !cloudvm.RunningOnAWS() {
		return nil
	}
	// Determine the IP address from VM's metadata
	if ip := cloudvm.ExternalIPAddress(); ip != nil {
		// 1:1 NAT case, our network config will not change.
		return &net.IPAddr{IP: ip}
	}
	return nil
}
