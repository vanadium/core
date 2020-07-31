// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

// VirtualizedFlags contains the values of the Virtualized flag group.
type VirtualizedFlags struct {
	Dockerized             bool            `cmdline:"v23.virtualized.docker,,set if the process is running in a docker container and needs to configure itself differently therein"`
	VirtualizationProvider string          `cmdline:"v23.virtualized.provider,,the name of the virtualization/cloud provider hosting this process if the process needs to configure itself differently therein"`
	DiscoverPublicIP       bool            `cmdline:"v23.virtualized.discover-public-address,,set if the process should attempt to discover the public IP address assigned to it by the virtualized environment it is running in"`
	PublicProtocol         TCPProtocolFlag `cmdline:"v23.virtualized.tcp.public-protocol,,if set the process will use this protocol for its entry in the mounttable"`
	PublicAddress          IPHostPortFlag  `cmdline:"v23.virtualized.tcp.public-address,,if set the process will use this address (resolving via dns if appropriate) for its entry in the mounttable"`
	LiteralDNSName         string          `cmdline:"v23.virtualized.dns.public-name,,if set the process will use the supplied dns name literally (ie. without resolution) for its entry in the mounttable"`
}

// VirtualizedFlagDefaults is used to set defaults for the Virtualized flag group.
type VirtualizedFlagDefaults struct {
	Dockerized             bool
	VirtualizationProvider string
	DiscoverPublicIP       bool
	PublicProtocol         string
	PublicAddress          string
	LiteralDNSName         string
}

// VirtualizationProvider identifies a particular virtualization provider/cloud
// computing vendor. Popular providers are defined here, but applications may
// chose to add define and act on others by creating additional runtime factories.
type VirtualizationProvider string

const (
	// AWS is reserved for Amazon Web Services.
	AWS VirtualizationProvider = "AWS"
	// GCP is reserved for Google's Compute Platform.
	GCP VirtualizationProvider = "GCP"
)
