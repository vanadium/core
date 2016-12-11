// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"log"

	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

const clientFactoryTmpl = header + `
// Source(s):  {{ .Sources }}
package {{ .PackagePath }};

/**
 * Factory for {@link {{ .ServiceName }}Client}s.
 */
public final class {{ .ServiceName }}ClientFactory {
    /**
     * Creates a new {@link {{ .ServiceName }}Client}, binding it to the provided name.
     *
     * @param name name to bind to
     */
    public static {{ .ServiceName }}Client get{{ .ServiceName }}Client(java.lang.String name) {
        return get{{ .ServiceName }}Client(name, null);
    }

    /**
     * Creates a new {@link {{ .ServiceName }}Client}, binding it to the provided name and using the
     * provided options.  Currently supported options are:
     * <p><ul>
     * <li>{@link io.v.v23.OptionDefs#CLIENT}, which specifies a {@link io.v.v23.rpc.Client} to use for all rpc calls.</li>
     * </ul>
     *
     * @param name name to bind to
     * @param opts creation options
     */
    public static {{ .ServiceName }}Client get{{ .ServiceName }}Client(java.lang.String name, io.v.v23.Options opts) {
        io.v.v23.rpc.Client client = null;
        if (opts != null && opts.get(io.v.v23.OptionDefs.CLIENT) != null) {
            client = opts.get(io.v.v23.OptionDefs.CLIENT, io.v.v23.rpc.Client.class);
        }
        return new {{ .ServiceName }}ClientImpl(client, name);
    }

    private {{ .ServiceName }}ClientFactory() {}
}
`

// genJavaClientFactoryFile generates the Java client factory file.
func genJavaClientFactoryFile(iface *compile.Interface, env *compile.Env) JavaFileInfo {
	javaServiceName := vdlutil.FirstRuneToUpper(iface.Name)
	data := struct {
		FileDoc     string
		Sources     string
		ServiceName string
		PackagePath string
	}{
		FileDoc:     iface.File.Package.FileDoc,
		Sources:     iface.File.BaseName,
		ServiceName: javaServiceName,
		PackagePath: javaPath(javaGenPkgPath(iface.File.Package.GenPath)),
	}
	var buf bytes.Buffer
	err := parseTmpl("client factory", clientFactoryTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute client template: %v", err)
	}
	return JavaFileInfo{
		Name: javaServiceName + "ClientFactory.java",
		Data: buf.Bytes(),
	}
}
