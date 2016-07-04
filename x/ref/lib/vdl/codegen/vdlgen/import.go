// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vdlgen implements VDL code generation from compiled VDL packages.
package vdlgen

// TODO(toddw): Add tests

import (
	"strconv"

	"v.io/x/ref/lib/vdl/codegen"
)

// Imports returns the vdl imports clause corresponding to imports; empty if
// there are no imports.
func Imports(imports codegen.Imports) string {
	var s string
	if len(imports) > 0 {
		s += "import ("
		for _, imp := range imports {
			s += "\n\t"
			if imp.Name != "" {
				s += imp.Name + " "
			}
			s += strconv.Quote(imp.Path)
		}
		s += "\n)"
	}
	return s
}
