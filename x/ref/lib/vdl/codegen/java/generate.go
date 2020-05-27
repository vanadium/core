// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package java implements Java code generation from compiled VDL packages.
package java

import (
	"path"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

// pkgPathXlator is the function used to translate a VDL package path
// into a Java package path.  If nil, no translation takes place.
var pkgPathXlator func(path string) string

// SetPkgPathXlator sets the function used to translate a VDL package
// path into a Java package path.
func SetPkgPathXlator(xlator func(path string) string) {
	pkgPathXlator = xlator
}

// javaGenPkgPath returns the Java package path given the VDL package path.
func javaGenPkgPath(vdlPkgPath string) string {
	if pkgPathXlator == nil {
		return vdlPkgPath
	}
	return pkgPathXlator(vdlPkgPath)
}

// JavaFileInfo stores the name and contents of the generated Java file.
//nolint:golint // API change required.
type JavaFileInfo struct {
	Dir  string
	Name string
	Data []byte
}

// Generate generates Java files for all VDL files in the provided package,
// returning the list of generated Java files as a slice.  Since Java requires
// that each public class/interface gets defined in a separate file, this method
// will return one generated file per struct.  (Interfaces actually generate
// two files because we create separate interfaces for clients and servers.)
// In addition, since Java doesn't support global variables (i.e., variables
// defined outside of a class), all constants are moved into a special "Consts"
// class and stored in a separate file.  All client bindings are stored in a
// separate Client.java file. Finally, package documentation (if any) is stored
// in a "package-info.java" file.
//
// TODO(spetrovic): Run Java formatters on the generated files.
func Generate(pkg *compile.Package, env *compile.Env) (ret []JavaFileInfo) {
	validateJavaConfig(pkg, env)
	// One file for package documentation (if any).
	if g := genJavaPackageFile(pkg, env); g != nil {
		ret = append(ret, *g)
	}
	// Single file for all constants' definitions.
	if g := genJavaConstFile(pkg, env); g != nil {
		ret = append(ret, *g)
	}
	for _, file := range pkg.Files {
		// Separate file for all typedefs.
		for _, tdef := range file.TypeDefs {
			switch tdef.Type.Kind() {
			case vdl.Array:
				ret = append(ret, genJavaArrayFile(tdef, env))
			case vdl.Enum:
				ret = append(ret, genJavaEnumFile(tdef, env))
			case vdl.List:
				ret = append(ret, genJavaListFile(tdef, env))
			case vdl.Map:
				ret = append(ret, genJavaMapFile(tdef, env))
			case vdl.Union:
				ret = append(ret, genJavaUnionFile(tdef, env))
			case vdl.Set:
				ret = append(ret, genJavaSetFile(tdef, env))
			case vdl.Struct:
				ret = append(ret, genJavaStructFile(tdef, env))
			default:
				ret = append(ret, genJavaPrimitiveFile(tdef, env))
			}
		}
		// Separate file for all interface definitions.
		for _, iface := range file.Interfaces {
			ret = append(ret, genJavaClientFactoryFile(iface, env))
			ret = append(ret, genJavaClientInterfaceFile(iface, env)) // client interface
			ret = append(ret, genJavaClientImplFile(iface, env))
			ret = append(ret, genJavaServerInterfaceFile(iface, env)) // server interface
			ret = append(ret, genJavaServerWrapperFile(iface, env))
		}
		for _, err := range file.ErrorDefs {
			// Separate file for all error defs.
			ret = append(ret, genJavaErrorFile(file, err, env))
		}
	}
	return
}

// The native types feature is hard to use correctly.  E.g. the wire type
// must be statically registered in Java vdl package in order for the
// wire<->native conversion to work, which is hard to ensure.
//
// Restrict the feature to these whitelisted VDL packages for now.
var nativeTypePackageWhitelist = map[string]bool{
	"time":                                   true,
	"v.io/v23/security":                      true,
	"v.io/v23/security/access":               true,
	"v.io/x/ref/lib/vdl/testdata/nativetest": true,
}

func validateJavaConfig(pkg *compile.Package, env *compile.Env) {
	vdlconfig := path.Join(pkg.GenPath, "vdl.config")
	// Validate native type configuration.  Since native types are hard to use, we
	// restrict them to a built-in whitelist of packages for now.
	if len(pkg.Config.Java.WireToNativeTypes) > 0 && !nativeTypePackageWhitelist[pkg.Path] {
		env.Errors.Errorf("%s: Java.WireToNativeTypes is restricted to whitelisted VDL packages", vdlconfig)
	}
}
