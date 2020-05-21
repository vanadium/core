// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"log"

	"v.io/x/ref/lib/vdl/compile"
)

const packageTmpl = header + `
// Source: {{ .Source }}

{{ .Doc }}
package {{ .PackagePath }};
`

// genPackageFileJava generates the Java package info file, iff any package
// comments were specified in the package's VDL files.
func genJavaPackageFile(pkg *compile.Package, env *compile.Env) *JavaFileInfo {
	generated := false
	for _, file := range pkg.Files {
		if file.PackageDef.Doc != "" {
			if generated {
				log.Printf("WARNING: Multiple vdl files with package documentation. One will be overwritten.")
				return nil
			}

			data := struct {
				Doc         string
				FileDoc     string
				PackagePath string
				Source      string
			}{
				Doc:         javaDoc(file.PackageDef.Doc, file.PackageDef.DocSuffix),
				FileDoc:     pkg.FileDoc,
				PackagePath: javaPath(javaGenPkgPath(pkg.GenPath)),
				Source:      javaFileNames(pkg.Files),
			}
			var buf bytes.Buffer
			err := parseTmpl("package", packageTmpl).Execute(&buf, data)
			if err != nil {
				log.Fatalf("vdl: couldn't execute package template: %v", err)
			}
			return &JavaFileInfo{
				Name: "package-info.java",
				Data: buf.Bytes(),
			}
		}
		generated = true
	}
	return nil
}
