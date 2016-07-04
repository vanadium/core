// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package control

import (
	"fmt"
	"sync"

	"v.io/x/ref/services/syncbase/longevity_tests/client"
)

// The client registry is used to map names to client implementations.
//
// If a Device model includes a client name, a client implementation will be
// looked up in the registry and started for that Device instance.
//
// Usage:
//
// fastWriter := func() *client.Writer {
//   return &Writer{
//     WriteInterval: 1 * time.Millisecond,
//   }
// }
// slowWriter := func() *client.Writer {
//   return &Writer{
//     WriteInterval: 10 * time.Second,
//   }
// }
//
// control.RegisterClient("fast-writer", fastWriter)
// control.RegisterClient("slow-writer", slowWriter)
//
// device := model.Device{
//    clients: []string{"fast-writer"}
//    ...
// }

var (
	clientRegistry   = map[string]ClientGenerator{}
	clientRegistryMu = sync.Mutex{}
)

// ClientGenerator is a function that generates a particular client
// implementation.
type ClientGenerator func() client.Client

// RegisterClient registers a ClientGenerator with the given name.
func RegisterClient(name string, c ClientGenerator) error {
	clientRegistryMu.Lock()
	defer clientRegistryMu.Unlock()
	if _, ok := clientRegistry[name]; ok {
		return fmt.Errorf("client already registered with name %q", name)
	}
	clientRegistry[name] = c
	return nil
}

// LookupClient returns a ClientGenerator from the registry.  If no
// ClientGenerator is registered under the given name, then LookupClient
// returns nil.
func LookupClient(name string) ClientGenerator {
	clientRegistryMu.Lock()
	defer clientRegistryMu.Unlock()
	return clientRegistry[name]
}
