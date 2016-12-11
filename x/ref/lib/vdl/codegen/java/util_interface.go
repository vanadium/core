// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"path"

	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

// allEmbeddedIfaces returns all unique interfaces in the embed tree
// starting at the provided interface (not including that interface).
func allEmbeddedIfaces(iface *compile.Interface) (ret []*compile.Interface) {
	added := make(map[string]bool)
	for _, eIface := range iface.Embeds {
		for _, eIface = range append(allEmbeddedIfaces(eIface), eIface) {
			path := path.Join(eIface.File.Package.GenPath, vdlutil.FirstRuneToUpper(eIface.Name))
			if added[path] { // already added iface
				continue
			}
			ret = append(ret, eIface)
			added[path] = true
		}
	}
	return
}

// interfaceFullyQualifiedName outputs the fully qualified name of an interface
// e.g. "com.a.B"
func interfaceFullyQualifiedName(iface *compile.Interface) string {
	return path.Join(javaGenPkgPath(iface.File.Package.GenPath), vdlutil.FirstRuneToUpper(iface.Name))
}

// javaClientExtendsStr creates an extends clause for a client interface
// e.g. "extends com.a.B, com.d.E"
func javaClientExtendsStr(embeds []*compile.Interface) string {
	if len(embeds) == 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString("extends ")
	for i, embed := range embeds {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(javaPath(interfaceFullyQualifiedName(embed)))
		buf.WriteString("Client")
	}
	return buf.String()
}

// javaServerExtendsStr creates an extends clause for a server interface
// e.g. "extends com.a.B, com.d.E"
func javaServerExtendsStr(embeds []*compile.Interface) string {
	if len(embeds) == 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteString("extends ")
	for i, embed := range embeds {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(javaPath(interfaceFullyQualifiedName(embed)))
		buf.WriteString("Server")
	}
	return buf.String()
}
