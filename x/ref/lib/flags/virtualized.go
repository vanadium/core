// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"fmt"
	"strings"
)

// VirtualizedFlags contains the values of the Virtualized flag group.
type VirtualizedFlags struct {
	Dockerized                bool                       `cmdline:"v23.virtualized.docker,,set if the process is running in a docker container and needs to configure itself differently therein"`
	VirtualizationProvider    VirtualizationProviderFlag `cmdline:"v23.virtualized.provider,,the name of the virtualization/cloud provider hosting this process if the process needs to configure itself differently therein"`
	DissallowNativeFallback   bool                       `cmdline:"v23.virtualized.disallow-native-fallback,,'if set, a failure to detect the requested virtualization provider will result in an error, otherwise, native mode is used'"`
	PublicProtocol            TCPProtocolFlag            `cmdline:"v23.virtualized.tcp.public-protocol,,if set the process will use this protocol for its entry in the mounttable"`
	PublicAddress             IPHostPortFlag             `cmdline:"v23.virtualized.tcp.public-address,,if set the process will use this address (resolving via dns if appropriate) for its entry in the mounttable"`
	PublicDNSName             HostPortFlag               `cmdline:"v23.virtualized.dns.public-name,,if set the process will use the supplied dns name (and port) without resolution for its entry in the mounttable"`
	AdvertisePrivateAddresses bool                       `cmdline:"v23.virtualized.advertise-private-addresses,,if set the process will also advertise its private addresses"`
}

// VirtualizedFlagDefaults is used to set defaults for the Virtualized flag group.
type VirtualizedFlagDefaults struct {
	Dockerized                bool
	VirtualizationProvider    string
	PublicProtocol            string
	PublicAddress             string
	PublicDNSName             string
	AdvertisePrivateAddresses bool
}

// VirtualizationProviderFlag represents
type VirtualizationProviderFlag struct {
	Provider VirtualizationProvider
}

// Set implements flat.Value.
func (vp *VirtualizationProviderFlag) Set(v string) error {
	switch {
	case strings.ToLower(v) == "aws":
		vp.Provider = AWS
		return nil
	case strings.ToLower(v) == "gcp":
		vp.Provider = GCP
		return nil
	case len(v) == 0:
		vp.Provider = Native
		return nil
	}
	return fmt.Errorf("unsupported virtualization provider: %v", v)
}

// String implements flat.Value.
func (vp *VirtualizationProviderFlag) String() string {
	switch vp.Provider {
	case AWS:
		return "AWS"
	case GCP:
		return "GCP"
	case Native:
		return ""
	}
	return ""
}

// Get implements flat.Getter.
func (vp VirtualizationProviderFlag) Get() interface{} {
	return vp.Provider
}

// VirtualizationProvider identifies a particular virtualization provider/cloud
// computing vendor. Popular providers are defined here, but applications may
// chose to add define and act on others by creating additional runtime factories.
type VirtualizationProvider string

const (
	// Native is reserved for any/all non-virtualized environments.
	Native VirtualizationProvider = ""
	// AWS is reserved for Amazon Web Services.
	AWS VirtualizationProvider = "AWS"
	// GCP is reserved for Google's Compute Platform.
	GCP VirtualizationProvider = "GCP"
)
