// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"fmt"

	"v.io/x/ref/lib/vdl/compile"
)

// javaDeclarationArgStr creates a comma separated string of args to be used in a function declaration
// e.g. "final int x, final Object o"
func javaDeclarationArgStr(args []*compile.Field, env *compile.Env, leadingComma bool) string {
	var buf bytes.Buffer
	for i, arg := range args {
		if leadingComma || i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString("final ")
		buf.WriteString(javaType(arg.Type, false, env))
		buf.WriteString(" ")
		if arg.Name != "" {
			buf.WriteString(arg.Name)
		} else {
			buf.WriteString(fmt.Sprintf("arg%d", i+1))
		}
	}
	return buf.String()
}

// javaCallingArgStr creates a comma separated string of arg to be used in calling a function
// e.g. "x, o"
func javaCallingArgStr(args []*compile.Field, leadingComma bool) string {
	var buf bytes.Buffer
	for i, arg := range args {
		if leadingComma || i > 0 {
			buf.WriteString(", ")
		}
		if arg.Name != "" {
			buf.WriteString(arg.Name)
		} else {
			buf.WriteString(fmt.Sprintf("arg%d", i+1))
		}
	}
	return buf.String()
}

// javaCallingArgTypeStr creates a comma separated string of arg types.
func javaCallingArgTypeStr(args []*compile.Field, env *compile.Env) string {
	var buf bytes.Buffer
	for i, arg := range args {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(javaReflectType(arg.Type, env))
	}
	return buf.String()
}
