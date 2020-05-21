// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlgen

// TODO(toddw): Add tests.

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
	"v.io/x/lib/textutil"
)

// NamedTypes represents a set of unique named types.  The main usage is to
// collect the named types from one or more signatures, and to print all of the
// named types.
type NamedTypes struct {
	types map[*vdl.Type]bool
}

// Add adds to x all named types from the type graph rooted at t.
func (x *NamedTypes) Add(t *vdl.Type) {
	if t == nil {
		return
	}
	t.Walk(vdl.WalkAll, func(t *vdl.Type) bool {
		if t.Name() != "" {
			if x.types == nil {
				x.types = make(map[*vdl.Type]bool)
			}
			x.types[t] = true
		}
		return true
	})
}

// Print pretty-prints x to w.
func (x NamedTypes) Print(w io.Writer) {
	var sorted typesByPkgAndName
	for t := range x.types {
		sorted = append(sorted, t)
	}
	sort.Sort(sorted)
	for _, t := range sorted {
		path, name := vdl.SplitIdent(t.Name())
		fmt.Fprintf(w, "\ntype %s %s\n", qualifiedName(path, name), BaseType(t, "", nil))
	}
}

// typesByPkgAndName orders types by package path, then by name.
type typesByPkgAndName []*vdl.Type

func (x typesByPkgAndName) Len() int      { return len(x) }
func (x typesByPkgAndName) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x typesByPkgAndName) Less(i, j int) bool {
	ipkg, iname := vdl.SplitIdent(x[i].Name())
	jpkg, jname := vdl.SplitIdent(x[j].Name())
	if ipkg != jpkg {
		return ipkg < jpkg
	}
	return iname < jname
}

// PrintInterface pretty-prints x to w.  The types are used to collect named
// types, so that they may be printed later.
func PrintInterface(w io.Writer, x signature.Interface, types *NamedTypes) {
	printDoc(w, x.Doc)
	fmt.Fprintf(w, "type %s interface {", qualifiedName(x.PkgPath, x.Name))
	indentW := textutil.ByteReplaceWriter(w, '\n', "\n\t")
	for _, embed := range x.Embeds {
		fmt.Fprint(indentW, "\n")
		PrintEmbed(indentW, embed)
	}
	for _, method := range x.Methods {
		fmt.Fprint(indentW, "\n")
		PrintMethod(indentW, method, types)
	}
	fmt.Fprintf(w, "\n}")
}

// PrintEmbed pretty-prints x to w.
func PrintEmbed(w io.Writer, x signature.Embed) {
	printDoc(w, x.Doc)
	fmt.Fprint(w, qualifiedName(x.PkgPath, x.Name))
}

// PrintMethod pretty-prints x to w.  The types are used to collect named types,
// so that they may be printed later.
func PrintMethod(w io.Writer, x signature.Method, types *NamedTypes) {
	printDoc(w, x.Doc)
	fmt.Fprintf(w, "%s(%s)%s%s%s", x.Name, inArgsStr(x.InArgs, types), streamArgsStr(x.InStream, x.OutStream, types), outArgsStr(x.OutArgs, types), tagsStr(x.Tags, types))
}

func inArgsStr(args []signature.Arg, types *NamedTypes) string {
	var ret []string
	for _, arg := range args {
		ret = append(ret, argStr(arg, types))
	}
	return strings.Join(ret, ", ")
}

func outArgsStr(args []signature.Arg, types *NamedTypes) string {
	if len(args) == 0 {
		return " error"
	}
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = argStr(arg, types)
	}
	return fmt.Sprintf(" (%s | error)", strings.Join(out, ", "))
}

func argStr(arg signature.Arg, types *NamedTypes) string {
	// TODO(toddw): Print arg.Doc somewhere?
	var ret string
	if arg.Name != "" {
		ret += arg.Name + " "
	}
	types.Add(arg.Type)
	return ret + Type(arg.Type, "", nil)
}

func streamArgsStr(in, out *signature.Arg, types *NamedTypes) string {
	if in == nil && out == nil {
		return ""
	}
	return fmt.Sprintf(" stream<%s, %s>", streamArgStr(in, types), streamArgStr(out, types))
}

func streamArgStr(arg *signature.Arg, types *NamedTypes) string {
	// TODO(toddw): Print arg.Name and arg.Doc?
	if arg == nil {
		return "_"
	}
	types.Add(arg.Type)
	return Type(arg.Type, "", nil)
}

func tagsStr(tags []*vdl.Value, types *NamedTypes) string {
	if len(tags) == 0 {
		return ""
	}
	var ret []string
	for _, tag := range tags {
		types.Add(tag.Type())
		ret = append(ret, TypedConst(tag, "", nil))
	}
	return fmt.Sprintf(" {%s}", strings.Join(ret, ", "))
}

func qualifiedName(pkgpath, name string) string {
	if pkgpath == "" {
		if name == "" {
			return "<empty>"
		}
		return name
	}
	return fmt.Sprintf("%q.%s", pkgpath, name)
}

func printDoc(w io.Writer, doc string) {
	if doc == "" {
		return
	}
	// TODO(toddw): Normalize the doc strings so that it never has the // or /**/
	// comment markers, and add them consistently here.  And use WrapWriter to
	// pretty-print the doc.
	if !strings.HasPrefix(doc, "//") {
		fmt.Fprintln(w, "// "+doc)
	} else {
		fmt.Fprintln(w, doc)
	}
}
