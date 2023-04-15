// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vdlroot defines the standard VDL packages; the VDLROOT environment
// variable should point at this directory when bootstrapping (see
// ./vdlroot/bootstrapping.go) for an explanation of bootstrapping.
//
// This package contains import dependencies on all its sub-packages.  This is
// meant as a convenient mechanism to pull in all standard vdl packages; import
// vdlroot to ensure the types for all standard vdl packages are registered.
//
// If VDLROOT is not specified, the .vdl and vdl.config files embedded in this
// package are used by the vdl tool chain.

//go:build !vdltoolbootstrapping && !vdlbootstrapping

package vdlroot

import (
	_ "v.io/v23/vdlroot/math"      //nolint:revive
	_ "v.io/v23/vdlroot/signature" //nolint:revive
	_ "v.io/v23/vdlroot/time"      //nolint:revive
	_ "v.io/v23/vdlroot/vdltool"   //nolint:revive
)
