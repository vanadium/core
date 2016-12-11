// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"v.io/v23/vdl"
)

func typedArg(name string, tt *vdl.Type) namedArg {
	return namedArg{name, tt.Kind() == vdl.Optional}
}

// namedArg represents a named argument, with methods to conveniently return the
// pointer or non-pointer form of the argument.
type namedArg struct {
	Name  string // variable name
	IsPtr bool   // is the variable a pointer type
}

func (arg namedArg) IsValid() bool {
	return arg.Name != ""
}

func (arg namedArg) Ptr() string {
	if arg.IsPtr {
		return arg.Name
	}
	return "&" + arg.Name
}

func (arg namedArg) Ref() string {
	if arg.IsPtr {
		return "*" + arg.Name
	}
	return arg.Name
}

func (arg namedArg) SafeRef() string {
	if arg.IsPtr {
		return "(*" + arg.Name + ")"
	}
	return arg.Name
}

func (arg namedArg) Field(field vdl.Field) namedArg {
	return typedArg(arg.Name+"."+field.Name, field.Type)
}

func (arg namedArg) Index(index string, tt *vdl.Type) namedArg {
	return typedArg(arg.SafeRef()+"["+index+"]", tt)
}

func (arg namedArg) ArrayIndex(index string, tt *vdl.Type) namedArg {
	// Go has a special-case: pointers to arrays can be indexed without the star.
	return typedArg(arg.Name+"["+index+"]", tt)
}
