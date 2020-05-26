// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"fmt"
	"log"
	"path"

	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

const serverInterfaceTmpl = header + `
// Source: {{ .Source }}
package {{ .PackagePath }};

{{ .ServerDoc }}
@io.v.v23.vdl.VServer(
    serverWrapper = {{ .ServerWrapperPath }}.class
)
public interface {{ .ServiceName }}Server {{ .Extends }} {
{{ range $method := .Methods }}
    {{/* If this method has multiple return arguments, generate the class. */}}
    {{ if $method.IsMultipleRet }}
    /**
     * Multi-return value for method {@link #{{$method.Name}}}.
     */
    @io.v.v23.vdl.MultiReturn
    public static class {{ $method.UppercaseMethodName }}Out {
        {{ range $retArg := $method.RetArgs }}
        public {{ $retArg.Type }} {{ $retArg.Name }};
        {{ end }}
    }
    {{ end }}

    {{/* Generate the method signature. */}}
    {{ $method.Doc }}
    @javax.annotation.CheckReturnValue
    com.google.common.util.concurrent.ListenableFuture<{{ $method.RetType }}> {{ $method.Name }}(io.v.v23.context.VContext context, io.v.v23.rpc.ServerCall call{{ $method.Args }});
{{ end }}
}
`

func serverInterfaceOutArg(iface *compile.Interface, method *compile.Method, env *compile.Env) string {
	switch len(method.OutArgs) {
	case 0:
		return "java.lang.Void"
	case 1:
		return javaType(method.OutArgs[0].Type, true, env)
	default:
		return javaPath(path.Join(interfaceFullyQualifiedName(iface)+"Server", method.Name+"Out"))
	}
}

func serverInterfaceStreamingArgType(method *compile.Method, env *compile.Env) string {
	sendType := javaType(method.OutStream, true, env)
	recvType := javaType(method.InStream, true, env)
	switch {
	case method.OutStream != nil && method.InStream != nil:
		return fmt.Sprintf("io.v.v23.vdl.ServerStream<%s, %s>", sendType, recvType)
	case method.OutStream != nil:
		return fmt.Sprintf("io.v.v23.vdl.ServerSendStream<%s>", sendType)
	case method.InStream != nil:
		return fmt.Sprintf("io.v.v23.vdl.ServerRecvStream<%s>", recvType)
	default:
		panic(fmt.Errorf("Streaming method without stream sender and receiver: %v", method))
	}
}

type serverInterfaceArg struct {
	Type string
	Name string
}

type serverInterfaceMethod struct {
	Args                string
	Doc                 string
	Name                string
	IsMultipleRet       bool
	RetArgs             []serverInterfaceArg
	RetType             string
	UppercaseMethodName string
}

func processServerInterfaceMethod(method *compile.Method, iface *compile.Interface, env *compile.Env) serverInterfaceMethod {
	args := javaDeclarationArgStr(method.InArgs, env, true)
	if isStreamingMethod(method) {
		args += ", " + serverInterfaceStreamingArgType(method, env) + " stream"
	}
	retArgs := make([]serverInterfaceArg, len(method.OutArgs))
	for i := 0; i < len(method.OutArgs); i++ {
		if method.OutArgs[i].Name != "" {
			retArgs[i].Name = vdlutil.FirstRuneToLower(method.OutArgs[i].Name)
		} else {
			retArgs[i].Name = fmt.Sprintf("ret%d", i+1)
		}
		retArgs[i].Type = javaType(method.OutArgs[i].Type, false, env)
	}
	return serverInterfaceMethod{
		Args:                args,
		Doc:                 javaDoc(method.Doc, method.DocSuffix),
		Name:                vdlutil.FirstRuneToLower(method.Name),
		IsMultipleRet:       len(retArgs) > 1,
		RetArgs:             retArgs,
		RetType:             serverInterfaceOutArg(iface, method, env),
		UppercaseMethodName: method.Name,
	}
}

// genJavaServerInterfaceFile generates the Java interface file for the provided
// interface.
func genJavaServerInterfaceFile(iface *compile.Interface, env *compile.Env) JavaFileInfo {
	methods := make([]serverInterfaceMethod, len(iface.Methods))
	for i, method := range iface.Methods {
		methods[i] = processServerInterfaceMethod(method, iface, env)
	}
	javaServiceName := vdlutil.FirstRuneToUpper(iface.Name)
	data := struct {
		FileDoc           string
		Extends           string
		Methods           []serverInterfaceMethod
		PackagePath       string
		ServerDoc         string
		ServerVDLPath     string
		ServiceName       string
		ServerWrapperPath string
		Source            string
	}{
		FileDoc:           iface.File.Package.FileDoc,
		Extends:           javaServerExtendsStr(iface.Embeds),
		Methods:           methods,
		PackagePath:       javaPath(javaGenPkgPath(iface.File.Package.GenPath)),
		ServerDoc:         javaDoc(iface.Doc, iface.DocSuffix),
		ServiceName:       javaServiceName,
		ServerVDLPath:     path.Join(iface.File.Package.GenPath, iface.Name+"ServerMethods"),
		ServerWrapperPath: javaPath(javaGenPkgPath(path.Join(iface.File.Package.GenPath, javaServiceName+"ServerWrapper"))),
		Source:            iface.File.BaseName,
	}
	var buf bytes.Buffer
	err := parseTmpl("server interface", serverInterfaceTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute struct template: %v", err)
	}
	return JavaFileInfo{
		Name: javaServiceName + "Server.java",
		Data: buf.Bytes(),
	}
}
