// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"log"
	"path"
	"strings"

	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

const serverWrapperTmpl = header + `
// Source(s):  {{ .Source }}
package {{ .PackagePath }};

/**
 * Wrapper for {@link {{ .ServiceName }}Server}.  This wrapper is used by
 * {@link io.v.v23.rpc.ReflectInvoker} to indirectly invoke server methods.
 */
public final class {{ .ServiceName }}ServerWrapper {
    private final {{ .FullServiceName }}Server server;

{{/* Define fields to hold each of the embedded server wrappers*/}}
{{ range $embed := .Embeds }}
    {{/* e.g. private final com.somepackage.gen_impl.ArithStub stubArith; */}}
    private final {{ $embed.FullName }}ServerWrapper wrapper{{ $embed.Name }};
    {{ end }}

    /**
     * Creates a new {@link {{ .ServiceName }}ServerWrapper} to invoke the methods of the
     * provided server.
     *
     * @param server server whose methods are to be invoked
     */
    public {{ .ServiceName }}ServerWrapper({{ .FullServiceName }}Server server) {
        this.server = server;
        {{/* Initialize the embedded server wrappers */}}
        {{ range $embed := .Embeds }}
        this.wrapper{{ $embed.Name }} = new {{ $embed.FullName }}ServerWrapper(server);
        {{ end }}
    }

    /**
     * Returns a description of this server.
     */
    public io.v.v23.vdlroot.signature.Interface signature() {
        java.util.List<io.v.v23.vdlroot.signature.Embed> embeds = new java.util.ArrayList<io.v.v23.vdlroot.signature.Embed>();
        java.util.List<io.v.v23.vdlroot.signature.Method> methods = new java.util.ArrayList<io.v.v23.vdlroot.signature.Method>();
        {{ range $method := .Methods }}
        {
            java.util.List<io.v.v23.vdlroot.signature.Arg> inArgs = new java.util.ArrayList<io.v.v23.vdlroot.signature.Arg>();
            {{ range $arg := $method.CallingArgTypes }}
            inArgs.add(new io.v.v23.vdlroot.signature.Arg("", "", new io.v.v23.vdl.VdlTypeObject({{ $arg }})));
            {{ end }}
            java.util.List<io.v.v23.vdlroot.signature.Arg> outArgs = new java.util.ArrayList<io.v.v23.vdlroot.signature.Arg>();
            {{ range $arg := $method.RetJavaTypes }}
            outArgs.add(new io.v.v23.vdlroot.signature.Arg("", "", new io.v.v23.vdl.VdlTypeObject({{ $arg }})));
            {{ end }}
            java.util.List<io.v.v23.vdl.VdlAny> tags = new java.util.ArrayList<io.v.v23.vdl.VdlAny>();
            {{ range $tag := .Tags }}
            tags.add(new io.v.v23.vdl.VdlAny(io.v.v23.vdl.VdlValue.valueOf({{ $tag.Value }}, {{ $tag.Type }})));
            {{ end }}
            methods.add(new io.v.v23.vdlroot.signature.Method(
                "{{ $method.Name }}",
                "{{ $method.Doc }}",
                inArgs,
                outArgs,
                null,
                null,
                tags));
        }
        {{ end }}

        return new io.v.v23.vdlroot.signature.Interface("{{ .ServiceName }}", "{{ .PackagePath }}", "{{ .Doc }}", embeds, methods);
    }

    /**
     * Returns all tags associated with the provided method or {@code null} if the method isn't
     * implemented by this server.
     *
     * @param method method whose tags are to be returned
     */
    @SuppressWarnings("unused")
    public io.v.v23.vdl.VdlValue[] getMethodTags(java.lang.String method) throws io.v.v23.verror.VException {
        {{ range $methodName, $tags := .MethodTags }}
        if ("{{ $methodName }}".equals(method)) {
            try {
                return new io.v.v23.vdl.VdlValue[] {
                    {{ range $tag := $tags }} io.v.v23.vdl.VdlValue.valueOf({{ $tag.Value }}, {{ $tag.Type }}), {{ end }}
                };
            } catch (IllegalArgumentException e) {
                throw new io.v.v23.verror.VException(String.format("Couldn't get tags for method \"{{ $methodName }}\": %s", e.getMessage()));
            }
        }
        {{ end }}
        {{ range $embed := .Embeds }}
        {
            io.v.v23.vdl.VdlValue[] tags = this.wrapper{{ $embed.Name }}.getMethodTags(method);
            if (tags != null) {
                return tags;
            }
        }
        {{ end }}
        return null;  // method not found
    }

     {{/* Iterate over methods defined directly in the body of this server */}}
    {{ range $method := .Methods }}
    {{ $method.JavaDoc }}
    public com.google.common.util.concurrent.ListenableFuture<{{ $method.RetType }}> {{ $method.Name }}(io.v.v23.context.VContext _ctx, final io.v.v23.rpc.StreamServerCall _call{{ $method.DeclarationArgs }}) {
        {{ if $method.IsStreaming }}
            io.v.v23.vdl.ServerStream<{{ $method.SendType }}, {{ $method.RecvType }}> _stream = new io.v.v23.vdl.ServerStream<{{ $method.SendType }}, {{ $method.RecvType }}>() {
            @Override
            public com.google.common.util.concurrent.ListenableFuture<Void> send({{ $method.SendType }} _item) {
                java.lang.reflect.Type _type = {{ $method.StreamSendReflectType }};
                return _call.send(_item, _type);
            }
            @Override
            public com.google.common.util.concurrent.ListenableFuture<{{ $method.RecvType }}> recv() {
                java.lang.reflect.Type _type = {{ $method.StreamRecvReflectType }};
                return com.google.common.util.concurrent.Futures.transform(_call.recv(_type), new com.google.common.base.Function<Object, {{ $method.RecvType }}>() {
                    @Override
                    public {{ $method.RecvType }} apply(Object result) {
                        return ({{ $method.RecvType }}) result;
                    }
                });
            }
        };
        {{ end }} {{/* end if $method.IsStreaming */}}
        return this.server.{{ $method.Name }}(_ctx, _call {{ $method.CallingArgs }} {{ if $method.IsStreaming }} ,_stream {{ end }} );
    }
{{end}}

{{/* Iterate over methods from embedded servers and generate code to delegate the work */}}
{{ range $eMethod := .EmbedMethods }}
    {{ $eMethod.JavaDoc }}
    public com.google.common.util.concurrent.ListenableFuture<{{ $eMethod.RetType }}> {{ $eMethod.Name }}(io.v.v23.context.VContext ctx, io.v.v23.rpc.StreamServerCall call{{ $eMethod.DeclarationArgs }}) throws io.v.v23.verror.VException {
        {{/* e.g. return this.stubArith.cosine(ctx, call, [args], options) */}}
        return this.wrapper{{ $eMethod.IfaceName }}.{{ $eMethod.Name }}(ctx, call{{ $eMethod.CallingArgs }});
    }
{{ end }} {{/* end range .EmbedMethods */}}
}
`

type serverWrapperMethod struct {
	CallingArgs           string
	CallingArgTypes       []string
	DeclarationArgs       string
	Doc                   string
	IsStreaming           bool
	JavaDoc               string
	Name                  string
	RecvType              string
	RetType               string
	RetJavaTypes          []string
	SendType              string
	StreamRecvReflectType string
	StreamSendReflectType string
	Tags                  []methodTag
}

type serverWrapperEmbedMethod struct {
	CallingArgs     string
	DeclarationArgs string
	Doc             string
	IfaceName       string
	JavaDoc         string
	Name            string
	RetType         string
}

type serverWrapperEmbed struct {
	Name     string
	FullName string
}

type methodTag struct {
	Value string
	Type  string
}

// TODO(sjr): move this to somewhere in util_*.
func toJavaString(goString string) string {
	result := strings.ReplaceAll(goString, "\"", "\\\"")
	result = strings.ReplaceAll(result, "\n", "\" + \n\"")
	return result
}

func processServerWrapperMethod(iface *compile.Interface, method *compile.Method, env *compile.Env, tags []methodTag) serverWrapperMethod {
	callArgTypes := make([]string, len(method.InArgs))
	for i, arg := range method.InArgs {
		callArgTypes[i] = javaReflectType(arg.Type, env)
	}
	retArgTypes := make([]string, len(method.OutArgs))
	for i, arg := range method.OutArgs {
		retArgTypes[i] = javaReflectType(arg.Type, env)
	}
	return serverWrapperMethod{
		CallingArgs:           javaCallingArgStr(method.InArgs, true),
		CallingArgTypes:       callArgTypes,
		DeclarationArgs:       javaDeclarationArgStr(method.InArgs, env, true),
		Doc:                   toJavaString(method.Doc),
		IsStreaming:           isStreamingMethod(method),
		JavaDoc:               javaDoc(method.Doc, method.DocSuffix),
		Name:                  vdlutil.FirstRuneToLower(method.Name),
		RecvType:              javaType(method.InStream, true, env),
		RetType:               serverInterfaceOutArg(iface, method, env),
		RetJavaTypes:          retArgTypes,
		SendType:              javaType(method.OutStream, true, env),
		StreamRecvReflectType: javaReflectType(method.InStream, env),
		StreamSendReflectType: javaReflectType(method.OutStream, env),
		Tags:                  tags,
	}
}

func processServerWrapperEmbedMethod(iface *compile.Interface, embedMethod *compile.Method, env *compile.Env) serverWrapperEmbedMethod {
	return serverWrapperEmbedMethod{
		CallingArgs:     javaCallingArgStr(embedMethod.InArgs, true),
		DeclarationArgs: javaDeclarationArgStr(embedMethod.InArgs, env, true),
		IfaceName:       vdlutil.FirstRuneToUpper(iface.Name),
		JavaDoc:         javaDoc(embedMethod.Doc, embedMethod.DocSuffix),
		Name:            vdlutil.FirstRuneToLower(embedMethod.Name),
		RetType:         serverInterfaceOutArg(iface, embedMethod, env),
	}
}

// genJavaServerWrapperFile generates a java file containing a server wrapper for the specified
// interface.
func genJavaServerWrapperFile(iface *compile.Interface, env *compile.Env) JavaFileInfo {
	embeds := []serverWrapperEmbed{}
	for _, embed := range allEmbeddedIfaces(iface) {
		embeds = append(embeds, serverWrapperEmbed{
			Name:     vdlutil.FirstRuneToUpper(embed.Name),
			FullName: javaPath(javaGenPkgPath(path.Join(embed.File.Package.GenPath, vdlutil.FirstRuneToUpper(embed.Name)))),
		})
	}
	methodTags := make(map[string][]methodTag)
	// Copy method tags off of the interface.
	methods := make([]serverWrapperMethod, len(iface.Methods))
	for i, method := range iface.Methods {
		tags := make([]methodTag, len(method.Tags))
		for j, tag := range method.Tags {
			tags[j].Value = javaConstVal(tag, env)
			tags[j].Type = javaReflectType(tag.Type(), env)
		}
		methodTags[vdlutil.FirstRuneToLower(method.Name)] = tags
		methods[i] = processServerWrapperMethod(iface, method, env, tags)
	}
	embedMethods := []serverWrapperEmbedMethod{}
	for _, embedMao := range dedupedEmbeddedMethodAndOrigins(iface) {
		embedMethods = append(embedMethods, processServerWrapperEmbedMethod(embedMao.Origin, embedMao.Method, env))
	}
	javaServiceName := vdlutil.FirstRuneToUpper(iface.Name)
	data := struct {
		FileDoc         string
		EmbedMethods    []serverWrapperEmbedMethod
		Embeds          []serverWrapperEmbed
		FullServiceName string
		Methods         []serverWrapperMethod
		MethodTags      map[string][]methodTag
		PackagePath     string
		ServiceName     string
		Source          string
		Doc             string
	}{
		FileDoc:         iface.File.Package.FileDoc,
		EmbedMethods:    embedMethods,
		Embeds:          embeds,
		FullServiceName: javaPath(interfaceFullyQualifiedName(iface)),
		Methods:         methods,
		MethodTags:      methodTags,
		PackagePath:     javaPath(javaGenPkgPath(iface.File.Package.GenPath)),
		ServiceName:     javaServiceName,
		Source:          iface.File.BaseName,
		Doc:             toJavaString(iface.NamePos.Doc),
	}
	var buf bytes.Buffer
	err := parseTmpl("server wrapper", serverWrapperTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute server wrapper template: %v", err)
	}
	return JavaFileInfo{
		Name: javaServiceName + "ServerWrapper.java",
		Data: buf.Bytes(),
	}
}
