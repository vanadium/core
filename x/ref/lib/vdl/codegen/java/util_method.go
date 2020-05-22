// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"fmt"
	"sort"

	"v.io/x/ref/lib/vdl/compile"
)

func isStreamingMethod(method *compile.Method) bool {
	return method.InStream != nil || method.OutStream != nil
}

// methodAndOrigin is simply a pair of a method and its origin (the interface from which it came from)
// The usefulness of this pair of data is that methods can be defined on multiple interfaces and embeds can clash for the same method.
// Therefore, we need to keep track of the originating interface.
type methodAndOrigin struct {
	Method *compile.Method
	Origin *compile.Interface
}

// allMethodsAndOrigins constructs a list of all methods in an interface
// (including embedded interfaces) along with their corresponding origin interface.
func allMethodsAndOrigin(iface *compile.Interface) []methodAndOrigin {
	result := make([]methodAndOrigin, len(iface.Methods))
	for i, method := range iface.Methods {
		result[i] = methodAndOrigin{
			Method: method,
			Origin: iface,
		}
	}
	for _, embed := range iface.Embeds {
		result = append(result, allMethodsAndOrigin(embed)...)
	}
	return result
}

// dedupedEmbeddedMethodAndOrigins returns the set of methods only defined in embedded interfaces and dedupes methods with the same name.
// This is used to generate a set of methods for a given service that are not (re)defined in the interface body (and instead only in embeddings).
func dedupedEmbeddedMethodAndOrigins(iface *compile.Interface) []methodAndOrigin {
	ifaceMethods := map[string]bool{}
	for _, method := range iface.Methods {
		ifaceMethods[method.Name] = true
	}

	embeddedMao := map[string]methodAndOrigin{}
	for _, mao := range allMethodsAndOrigin(iface) {
		if _, found := ifaceMethods[mao.Method.Name]; found {
			continue
		}
		if _, found := embeddedMao[mao.Method.Name]; found {
			continue
		}
		embeddedMao[mao.Method.Name] = mao
	}

	ret := []methodAndOrigin{}
	for _, mao := range embeddedMao {
		ret = append(ret, mao)
	}
	sort.Sort(bySignature(ret))

	return ret
}

// bySignature implements sort.Interface
type bySignature []methodAndOrigin

func (b bySignature) Len() int {
	return len(b)
}

func (b bySignature) Less(i, j int) bool {
	return b.signature(i) < b.signature(j)
}

func (b bySignature) signature(i int) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s|%s|%s",
		b[i].Origin.File.Package.GenPath,
		b[i].Origin.Name,
		b[i].Method.Name,
	))
	for _, arg := range b[i].Method.InArgs {
		buf.WriteString(fmt.Sprintf("|%s", arg.Type.Name()))
	}
	return buf.String()
}

func (b bySignature) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
