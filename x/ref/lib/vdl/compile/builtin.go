// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
)

var (
	// BuiltInPackage and BuiltInFile are used to hold the built-ins.
	BuiltInPackage = newPackage("", "_builtin", "_builtin", vdltool.Config{})
	BuiltInFile    = &File{BaseName: "_builtin.vdl"}
)

func init() {
	// Link the BuiltIn{Package,File} to each other before defining built-ins.
	BuiltInPackage.Files = []*File{BuiltInFile}
	BuiltInFile.Package = BuiltInPackage
	// Built-in types
	builtInType("any", vdl.AnyType)
	builtInType("bool", vdl.BoolType)
	builtInType("byte", vdl.ByteType)
	builtInType("uint16", vdl.Uint16Type)
	builtInType("uint32", vdl.Uint32Type)
	builtInType("uint64", vdl.Uint64Type)
	builtInType("int8", vdl.Int8Type)
	builtInType("int16", vdl.Int16Type)
	builtInType("int32", vdl.Int32Type)
	builtInType("int64", vdl.Int64Type)
	builtInType("float32", vdl.Float32Type)
	builtInType("float64", vdl.Float64Type)
	builtInType("string", vdl.StringType)
	builtInType("typeobject", vdl.TypeObjectType)
	builtInType("error", vdl.ErrorType)
	// Built-in consts
	builtInConst("nil", NilConst)
	builtInConst("true", TrueConst)
	builtInConst("false", FalseConst)
}

func builtInType(name string, t *vdl.Type) {
	def := &TypeDef{
		NamePos:  NamePos{Name: name},
		Exported: true,
		Type:     t,
		File:     BuiltInFile,
	}
	addTypeDef(def, nil)
}

func builtInConst(name string, v *vdl.Value) {
	def := &ConstDef{
		NamePos:  NamePos{Name: name},
		Exported: true,
		Value:    v,
		File:     BuiltInFile,
	}
	addConstDef(def, nil)
}
