// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package version provides versioning for the agent.  A client is expected to
// be able to start and communicate with an agent that supports a compatible
// version.
package version

import (
	"fmt"
	"strconv"

	"v.io/v23/context"
	"v.io/v23/verror"
)

// TODO(caprita): This mimics the rpc version package at
// v23/rpc/version/version.go.  Consider coming up with a common,
// package-independent versioning mechanism that can be reused.

const pkgPath = "v.io/x/ref/services/agent/internal/version"

type T uint32

var (
	errNoCompatibleVersion = verror.Register(pkgPath+".errNoCompatibleVersion", verror.NoRetry, "{1:}{2:} There were no compatible versions between ({3},{4}) and ({5},{6})")
	errInvalidVersion      = verror.Register(pkgPath+".errInvalidVersion", verror.NoRetry, "{1:}{2:} Invalid version {3}{:_}")
)

// Make T implement flag.Value.

// String returns a string representation of version.
func (v T) String() string {
	return fmt.Sprint(uint32(v))
}

// Set parses the version from the given string.  An error is returned if the
// string appears in the wrong syntax or represents an invalid version number.
func (v *T) Set(s string) error {
	intV, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return err
	}
	*v = T(intV)
	return nil
}

// Range describes a range of versions.
type Range struct {
	Min, Max T
}

// Supported defines the version range supported by the current binary.
var Supported = Range{Min: 0, Max: 0}

// String returns a string representation of the range.
func (r Range) String() string {
	return fmt.Sprintf("[%v,%v]", r.Min, r.Max)
}

// Common returns the highest common version between the provided ranges.
// Returns an error if no such version exists.
func Common(ctx *context.T, l, r Range) (T, error) {
	minMax := l.Max
	if r.Max < minMax {
		minMax = r.Max
	}
	if l.Min > minMax || r.Min > minMax {
		return 0, verror.New(errNoCompatibleVersion, ctx,
			uint64(l.Min), uint64(l.Max), uint64(r.Min), uint64(r.Max))
	}
	return minMax, nil
}

// Contains returns whether the given version is in the given range.
func (r Range) Contains(v T) bool {
	return r.Max >= v && r.Min <= v
}

// RangeFromString parses a Range from the given string min, max values.
func RangeFromString(ctx *context.T, min, max string) (Range, error) {
	var vMin, vMax T
	if err := vMin.Set(min); err != nil {
		return Range{}, verror.New(errInvalidVersion, ctx, min, err)
	}
	if err := vMax.Set(max); err != nil {
		return Range{}, verror.New(errInvalidVersion, ctx, max, err)
	}
	return Range{vMin, vMax}, nil
}
