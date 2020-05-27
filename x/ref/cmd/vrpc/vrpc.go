// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/rpc/reserved"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/codegen/json"
	"v.io/x/ref/lib/vdl/codegen/vdlgen"
	"v.io/x/ref/lib/vdl/compile"

	_ "v.io/x/ref/runtime/factories/generic"
)

var (
	flagInsecure       bool
	flagShowReserved   bool
	flagShallowResolve bool
	flagJSON           bool
	insecureOpts       = []rpc.CallOpt{
		options.ServerAuthorizer{Authorizer: security.AllowEveryone()},
		options.NameResolutionAuthorizer{Authorizer: security.AllowEveryone()},
	}
)

func main() {
	cmdline.HideGlobalFlagsExcept(regexp.MustCompile(`^v23\.namespace\.root$`))
	cmdline.Main(cmdVRPC)
}

func init() {
	const (
		insecureVal  = false
		insecureName = "insecure"
		insecureDesc = "If true, skip server authentication. This means that the client will reveal its blessings to servers that it may not recognize."
	)
	cmdSignature.Flags.BoolVar(&flagInsecure, insecureName, insecureVal, insecureDesc)
	cmdIdentify.Flags.BoolVar(&flagInsecure, insecureName, insecureVal, insecureDesc)

	cmdSignature.Flags.BoolVar(&flagShowReserved, "show-reserved", false, "if true, also show the signatures of reserved methods")
	cmdVRPC.Flags.BoolVar(&flagShallowResolve, "s", false, "if true, perform a shallow resolve")

	cmdVRPC.Flags.BoolVar(&flagJSON, "json", false, "if true, output a JSON representation of the response")
}

var cmdVRPC = &cmdline.Command{
	Name:  "vrpc",
	Short: "sends and receives Vanadium remote procedure calls",
	Long: `
Command vrpc sends and receives Vanadium remote procedure calls.  It is used as
a generic client to interact with any Vanadium server.
`,
	// TODO(toddw): Add cmdServe, which will take an interface as input, and set
	// up a server capable of handling the given methods.  When a request is
	// received, it'll allow the user to respond via stdin.
	Children: []*cmdline.Command{cmdSignature, cmdCall, cmdIdentify},
}

const serverDesc = `
<server> identifies a Vanadium server.  It can either be the object address of
the server, or an object name that will be resolved to an end-point.
`

var cmdSignature = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runSignature),
	Name:   "signature",
	Short:  "Describe the interfaces of a Vanadium server",
	Long: `
Signature connects to the Vanadium server identified by <server>.

If no [method] is provided, returns all interfaces implemented by the server.

If a [method] is provided, returns the signature of just that method.
`,
	ArgsName: "<server> [method]",
	ArgsLong: serverDesc + `
[method] is the optional server method name.
`,
}

var cmdCall = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runCall),
	Name:   "call",
	Short:  "Call a method of a Vanadium server",
	Long: `
Call connects to the Vanadium server identified by <server> and calls the
<method> with the given positional [args...], returning results on stdout.

TODO(toddw): stdin is read for streaming arguments sent to the server.  An EOF
on stdin (e.g. via ^D) causes the send stream to be closed.

Regardless of whether the call is streaming, the main goroutine blocks for
streaming and positional results received from the server.

All input arguments (both positional and streaming) are specified as VDL
expressions, with commas separating multiple expressions.  Positional arguments
may also be specified as separate command-line arguments.  Streaming arguments
may also be specified as separate newline-terminated expressions.

The method signature is always retrieved from the server as a first step.  This
makes it easier to input complex typed arguments, since the top-level type for
each argument is implicit and doesn't need to be specified.
`,
	ArgsName: "<server> <method> [args...]",
	ArgsLong: serverDesc + `
<method> is the server method to call.

[args...] are the positional input arguments, specified as VDL expressions.
`,
}

var cmdIdentify = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runIdentify),
	Name:   "identify",
	Short:  "Reveal blessings presented by a Vanadium server",
	Long: `
Identify connects to the Vanadium server identified by <server> and dumps out
the blessings presented by that server (and the subset of those that are
considered valid by the principal running this tool) to standard output.
`,
	ArgsName: "<server>",
	ArgsLong: serverDesc,
}

func getNamespaceOpts(opts []rpc.CallOpt) []naming.NamespaceOpt {
	var out []naming.NamespaceOpt
	for _, o := range opts {
		if co, ok := o.(naming.NamespaceOpt); ok {
			out = append(out, co)
		}
	}
	return out
}

func rpcOpts(ctx *context.T, server string) ([]rpc.CallOpt, error) {
	var opts []rpc.CallOpt
	if flagInsecure {
		// Note that this flag is only settable on signature and
		// identify, as per ashankar@.
		opts = append(opts, insecureOpts...)
	}
	if flagShallowResolve {
		// Find the containing mount table.
		me, err := v23.GetNamespace(ctx).ShallowResolve(ctx, server, getNamespaceOpts(opts)...)
		if err != nil {
			return nil, err
		}
		opts = append(opts, options.Preresolved{Resolution: me})
	}
	return opts, nil
}

func runSignature(ctx *context.T, env *cmdline.Env, args []string) error {
	// Error-check args.
	var server, method string
	switch len(args) {
	case 1:
		server = args[0]
	case 2:
		server, method = args[0], args[1]
	default:
		return env.UsageErrorf("wrong number of arguments")
	}
	// Get the interface or method signature, and pretty-print.  We print the
	// named types after the signatures, to aid in readability.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	var types vdlgen.NamedTypes
	opts, err := rpcOpts(ctx, server)
	if err != nil {
		return err
	}
	if method != "" {
		methodSig, err := reserved.MethodSignature(ctx, server, method, opts...)
		if err != nil {
			return fmt.Errorf("MethodSignature failed: %v", err)
		}
		vdlgen.PrintMethod(env.Stdout, methodSig, &types)
		fmt.Fprintln(env.Stdout)
		types.Print(env.Stdout)
		return nil
	}
	ifacesSig, err := reserved.Signature(ctx, server, opts...)
	if err != nil {
		return fmt.Errorf("Signature failed: %v", err)
	}
	for i, iface := range ifacesSig {
		if !flagShowReserved && naming.IsReserved(iface.Name) {
			continue
		}
		if i > 0 {
			fmt.Fprintln(env.Stdout)
		}
		vdlgen.PrintInterface(env.Stdout, iface, &types)
		fmt.Fprintln(env.Stdout)
	}
	types.Print(env.Stdout)
	return nil
}

func runCall(ctx *context.T, env *cmdline.Env, args []string) error { //nolint:gocyclo
	// Error-check args, and set up argsdata with a comma-separated list of
	// arguments, allowing each individual arg to already be comma-separated.
	//
	// TODO(toddw): Should we just space-separate the args instead?
	if len(args) < 2 {
		return env.UsageErrorf("must specify <server> and <method>")
	}
	server, method := args[0], args[1]
	var argsdata string
	for _, arg := range args[2:] {
		arg := strings.TrimSpace(arg)
		if argsdata == "" || strings.HasSuffix(argsdata, ",") || strings.HasPrefix(arg, ",") {
			argsdata += arg
		} else {
			argsdata += "," + arg
		}
	}
	opts, err := rpcOpts(ctx, server)
	if err != nil {
		return err
	}
	// Get the method signature and parse args.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	methodSig, err := reserved.MethodSignature(ctx, server, method, opts...)
	if err != nil {
		return fmt.Errorf("MethodSignature failed: %v", err)
	}
	inargs, err := parseInArgs(argsdata, methodSig)
	if err != nil {
		// TODO: Print signature and example.
		return err
	}
	// Start the method call.
	call, err := v23.GetClient(ctx).StartCall(ctx, server, method, inargs, opts...)
	if err != nil {
		return fmt.Errorf("StartCall failed: %v", err)
	}
	// TODO(toddw): Fire off a goroutine to handle streaming inputs.
	// Handle streaming results.
StreamingResultsLoop:
	for {
		var item *vdl.Value
		switch err := call.Recv(&item); {
		case err == io.EOF:
			break StreamingResultsLoop
		case err != nil:
			return fmt.Errorf("call.Recv failed: %v", err)
		}
		if flagJSON {
			fmt.Fprintf(env.Stdout, "<< %v\n", json.Const(item, "", nil))
		} else {
			fmt.Fprintf(env.Stdout, "<< %v\n", vdlgen.TypedConst(item, "", nil))
		}
	}
	// Finish the method call.
	outargs := make([]*vdl.Value, len(methodSig.OutArgs))
	outptrs := make([]interface{}, len(outargs))
	for i := range outargs {
		outptrs[i] = &outargs[i]
	}
	if err := call.Finish(outptrs...); err != nil {
		return fmt.Errorf("call.Finish failed: %v", err)
	}
	// Pretty-print results.
	for i, arg := range outargs {
		if i > 0 {
			fmt.Fprint(env.Stdout, " ")
		}
		if flagJSON {
			fmt.Fprint(env.Stdout, json.Const(arg, "", nil))
		} else {
			fmt.Fprint(env.Stdout, vdlgen.TypedConst(arg, "", nil))
		}
	}
	fmt.Fprintln(env.Stdout)
	return nil
}

func parseInArgs(argsdata string, methodSig signature.Method) ([]interface{}, error) {
	if len(methodSig.InArgs) == 0 {
		return nil, nil
	}
	var intypes []*vdl.Type
	for _, inarg := range methodSig.InArgs {
		intypes = append(intypes, inarg.Type)
	}
	env := compile.NewEnv(-1)
	inargs := build.BuildExprs(argsdata, intypes, env)
	if err := env.Errors.ToError(); err != nil {
		return nil, fmt.Errorf("can't parse in-args:\n%v", err)
	}
	if got, want := len(inargs), len(methodSig.InArgs); got != want {
		return nil, fmt.Errorf("got %d args, want %d", got, want)
	}
	// Translate []*vdl.Value to []interface, with each item still *vdl.Value.
	var ret []interface{}
	for _, arg := range inargs {
		ret = append(ret, arg)
	}
	return ret, nil
}

func runIdentify(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(args) != 1 {
		return env.UsageErrorf("wrong number of arguments")
	}
	server := args[0]
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	opts, err := rpcOpts(ctx, server)
	if err != nil {
		return err
	}
	// The method name does not matter - only interested in authentication,
	// not in actually making an RPC.
	call, err := v23.GetClient(ctx).StartCall(ctx, server, "", nil, opts...)
	if err != nil {
		return fmt.Errorf(`client.StartCall(%q, "", nil) failed with %v`, server, err)
	}
	valid, presented := call.RemoteBlessings()
	pkey, _ := presented.PublicKey().MarshalBinary()
	fmt.Fprintf(env.Stdout, "PRESENTED: %v\nVALID:     %v\nPUBLICKEY: %v\n           %v\n", presented, valid, presented.PublicKey(), base64.URLEncoding.EncodeToString(pkey))
	return nil
}
