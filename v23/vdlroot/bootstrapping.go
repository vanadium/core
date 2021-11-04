// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build vdlbootstrapping || vdltoolbootstrapping
// +build vdlbootstrapping vdltoolbootstrapping

package vdlroot

import (
	_ "v.io/v23/vdlroot/vdltool" //nolint:revive
)
