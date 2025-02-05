// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import "v.io/v23/verror"

const (
	// UnknownProtocol, the empty string, is used when a protocol is specified
	// in an Endpoint.
	UnknownProtocol = ""
)

var (
	ErrNameExists              = verror.NewID("v.io/v23/naming.nameExists")
	ErrNoSuchName              = verror.NewID("v.io/v23/naming.nameDoesntExist")
	ErrNoSuchNameRoot          = verror.NewID("v.io/v23/naming.rootNameDoesntExist")
	ErrResolutionDepthExceeded = verror.NewID("v.io/v23/naming.resolutionDepthExceeded")
	ErrNoMountTable            = verror.NewID("v.io/v23/naming.noMounttable")
)

// Names returns the servers represented by MountEntry as names, including
// the MountedName suffix.
func (e *MountEntry) Names() []string {
	var names []string
	for _, s := range e.Servers {
		names = append(names, JoinAddressName(s.Server, e.Name))
	}
	return names
}

// CacheCtl is a cache control for the resolution cache.
type CacheCtl interface {
	CacheCtl()
}

// DisbleCache disables the resolution cache when set to true and enables if false.
// As a side effect one can flush the cache by disabling and then reenabling it.
type DisableCache bool

func (DisableCache) CacheCtl() {}

// NamespaceOpt is the interface for all Namespace options.
type NamespaceOpt interface {
	NSOpt()
}

// ReplaceMount requests the mount to replace the previous mount.
type ReplaceMount bool

func (ReplaceMount) NSOpt() {}

// ServesMountTable means the target is a mount table.
type ServesMountTable bool

func (ServesMountTable) NSOpt()       {}
func (ServesMountTable) EndpointOpt() {}

// IsLeaf means the target is a leaf
type IsLeaf bool

func (IsLeaf) NSOpt() {}

// BlessingOpt is used to add a blessing name to the endpoint.
type BlessingOpt string

func (BlessingOpt) EndpointOpt() {}

// RouteOpt is used to add a route to the endpoint.
type RouteOpt string

func (RouteOpt) EndpointOpt() {}

// When this prefix is present at the beginning of an object name suffix, the
// server may intercept the request and handle it internally. This is used to
// provide debugging, monitoring and other common functionality across all
// servers. Applications cannot use any name component that starts with this
// prefix.
const ReservedNamePrefix = "__"
