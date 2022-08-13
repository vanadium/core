// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlroot

import (
	"embed"
)

// Builtin contains the builtin vdl and vdl.config files used by
// the vdl compiler.
//
//go:embed */*.vdl */vdl.config
var Builtin embed.FS
