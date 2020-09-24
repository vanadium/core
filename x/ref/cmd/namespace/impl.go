// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"v.io/x/lib/cmdline"
	"v.io/x/lib/set"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/static"
)

func main() {
	cmdline.HideGlobalFlagsExcept(regexp.MustCompile(`^v23\.namespace\.root$`))
	cmdline.Main(cmdRoot)
}

var (
	flagLongGlob            bool
	flagInsecureResolve     bool
	flagInsecureResolveToMT bool
	flagDeleteSubtree       bool
	flagShallowResolve      bool
	flagErrorOnAnyError     bool
)

func init() {
	cmdGlob.Flags.BoolVar(&flagLongGlob, "l", false, "Long listing format.")
	cmdGlob.Flags.BoolVar(&flagErrorOnAnyError, "fail-on-any-error", false, "exit on any error, by default glob will report errors but not return a non-zero exit code")
	cmdResolve.Flags.BoolVar(&flagInsecureResolve, "insecure", false, "Insecure mode: May return results from untrusted servers and invoke Resolve on untrusted mounttables")
	cmdResolve.Flags.BoolVar(&flagShallowResolve, "s", false, "True to perform a shallow resolution")
	cmdResolveToMT.Flags.BoolVar(&flagInsecureResolveToMT, "insecure", false, "Insecure mode: May return results from untrusted servers and invoke Resolve on untrusted mounttables")
	cmdDelete.Flags.BoolVar(&flagDeleteSubtree, "r", false, "Delete all children of the name in addition to the name itself.")

}

var cmdGlob = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runGlob),
	Name:     "glob",
	Short:    "Returns all matching entries from the namespace",
	Long:     "Returns all matching entries from the namespace.",
	ArgsName: "<pattern>",
	ArgsLong: `
<pattern> is a glob pattern that is matched against all the names below the
specified mount name.
`,
}

func handleErrors(successes int, errors []*naming.GlobError) error {
	if len(errors) == 0 {
		return nil
	}
	if !flagErrorOnAnyError && successes > 0 {
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "%s: %v\n", err.Name, err.Error)
		}
		return nil
	}
	msg := ""
	for _, err := range errors {
		msg += fmt.Sprintf("%s: %v\n", err.Name, err.Error)
	}
	return fmt.Errorf("%v", strings.TrimSuffix(msg, "\n"))
}

func runGlob(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("glob: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	pattern := args[0]

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ns := v23.GetNamespace(ctx)

	c, err := ns.Glob(ctx, pattern)
	if err != nil {
		ctx.Infof("ns.Glob(%q) failed: %v", pattern, err)
		return err
	}
	errors := []*naming.GlobError{}
	successes := 0
	if flagLongGlob {
		// Show all the information we received.
		for res := range c {
			switch v := res.(type) {
			case *naming.GlobReplyEntry:
				fmt.Fprint(env.Stdout, v.Value.Name)
				for _, s := range v.Value.Servers {
					delta := time.Until(s.Deadline.Time)
					fmt.Fprintf(env.Stdout, " %s (Expires in %d sec)", s.Server, int(delta.Seconds()))
				}
				fmt.Fprintln(env.Stdout)
				successes++
			case *naming.GlobReplyError:
				errors = append(errors, &v.Value)
			}
		}
		return handleErrors(successes, errors)
	}
	// Show a sorted list of unique names, and any errors.
	resultSet := make(map[string]struct{})
	for res := range c {
		switch v := res.(type) {
		case *naming.GlobReplyEntry:
			if v.Value.Name != "" {
				resultSet[v.Value.Name] = struct{}{}
			}
			successes++
		case *naming.GlobReplyError:
			errors = append(errors, &v.Value)
		}
	}
	results := set.String.ToSlice(resultSet)
	sort.Strings(results)
	for _, result := range results {
		fmt.Fprintln(env.Stdout, result)
	}
	return handleErrors(successes, errors)
}

var cmdMount = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runMount),
	Name:     "mount",
	Short:    "Adds a server to the namespace",
	Long:     "Adds server <server> to the namespace with name <name>.",
	ArgsName: "<name> <server> <ttl>",
	ArgsLong: `
<name> is the name to add to the namespace.
<server> is the object address of the server to add.
<ttl> is the TTL of the new entry. It is a decimal number followed by a unit
suffix (s, m, h). A value of 0s represents an infinite duration.
`,
}

func runMount(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 3, len(args); expected != got {
		return env.UsageErrorf("mount: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	server := args[1]
	ttlArg := args[2]

	ttl, err := time.ParseDuration(ttlArg)
	if err != nil {
		return fmt.Errorf("TTL parse error: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ns := v23.GetNamespace(ctx)
	if err = ns.Mount(ctx, name, server, ttl); err != nil {
		ctx.Infof("ns.Mount(%q, %q, %s) failed: %v", name, server, ttl, err)
		return err
	}
	fmt.Fprintln(env.Stdout, "Server mounted successfully.")
	return nil
}

var cmdUnmount = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runUnmount),
	Name:     "unmount",
	Short:    "Removes a server from the namespace",
	Long:     "Removes server <server> with name <name> from the namespace.",
	ArgsName: "<name> <server>",
	ArgsLong: `
<name> is the name to remove from the namespace.
<server> is the object address of the server to remove.
`,
}

func runUnmount(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("unmount: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	server := args[1]

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ns := v23.GetNamespace(ctx)

	if err := ns.Unmount(ctx, name, server); err != nil {
		ctx.Infof("ns.Unmount(%q, %q) failed: %v", name, server, err)
		return err
	}
	fmt.Fprintln(env.Stdout, "Server unmounted successfully.")
	return nil
}

var cmdResolve = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runResolve),
	Name:     "resolve",
	Short:    "Translates a object name to its object address(es)",
	Long:     "Translates a object name to its object address(es).",
	ArgsName: "<name>",
	ArgsLong: "<name> is the name to resolve.",
}

func runResolve(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("resolve: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ns := v23.GetNamespace(ctx)

	var opts []naming.NamespaceOpt
	if flagInsecureResolve {
		opts = append(opts, options.NameResolutionAuthorizer{
			Authorizer: security.AllowEveryone(),
		})
	}
	var err error
	var me *naming.MountEntry
	if flagShallowResolve {
		me, err = ns.ShallowResolve(ctx, name, opts...)
	} else {
		me, err = ns.Resolve(ctx, name, opts...)
	}
	if err != nil {
		ctx.Infof("ns.Resolve(%q) failed: %v", name, err)
		return err
	}
	for _, n := range me.Names() {
		fmt.Fprintln(env.Stdout, n)
	}
	return nil
}

var cmdResolveToMT = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runResolveToMT),
	Name:     "resolvetomt",
	Short:    "Finds the address of the mounttable that holds an object name",
	Long:     "Finds the address of the mounttable that holds an object name.",
	ArgsName: "<name>",
	ArgsLong: "<name> is the name to resolve.",
}

func runResolveToMT(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("resolvetomt: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ns := v23.GetNamespace(ctx)
	var opts []naming.NamespaceOpt
	if flagInsecureResolveToMT {
		opts = append(opts, options.NameResolutionAuthorizer{
			Authorizer: security.AllowEveryone(),
		})
	}
	e, err := ns.ResolveToMountTable(ctx, name, opts...)
	if err != nil {
		ctx.Infof("ns.ResolveToMountTable(%q) failed: %v", name, err)
		return err
	}
	for _, s := range e.Servers {
		fmt.Fprintln(env.Stdout, naming.JoinAddressName(s.Server, e.Name))
	}
	return nil
}

var cmdPermissions = &cmdline.Command{
	Name:  "permissions",
	Short: "Manipulates permissions on an entry in the namespace",
	Long: `
Commands to get and set the permissions on a name - controlling the blessing
names required to resolve the name.

The permissions are provided as an JSON-encoded version of the Permissions type
defined in v.io/v23/security/access/types.vdl.
`,
	Children: []*cmdline.Command{cmdPermissionsGet, cmdPermissionsSet},
}

var cmdPermissionsSet = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runPermissionsSet),
	Name:   "set",
	Short:  "Sets permissions on a mount name",
	Long: `
Set replaces the permissions controlling usage of a mount name.
`,
	ArgsName: "<name> <permissions>",
	ArgsLong: `
<name> is the name on which permissions are to be set.

<permissions> is the path to a file containing a JSON-encoded Permissions
object (defined in v.io/v23/security/access/types.vdl), or "-" for STDIN.
`,
}

func runPermissionsSet(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("set: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	var perms access.Permissions
	file := os.Stdin
	if args[1] != "-" {
		var err error
		if file, err = os.Open(args[1]); err != nil {
			return err
		}
		defer file.Close()
	}
	if err := json.NewDecoder(file).Decode(&perms); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	ns := v23.GetNamespace(ctx)
	for {
		_, etag, err := ns.GetPermissions(ctx, name)
		if err != nil && !errors.Is(err, naming.ErrNoSuchName) {
			return err
		}
		if err = ns.SetPermissions(ctx, name, perms, etag); errors.Is(err, verror.ErrBadVersion) {
			ctx.Infof("SetPermissions(%q, %q) failed: %v, retrying...", name, etag, err)
			continue
		}
		return err
	}
}

var cmdPermissionsGet = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runPermissionsGet),
	Name:     "get",
	Short:    "Gets permissions on a mount name",
	ArgsName: "<name>",
	ArgsLong: `
<name> is a name in the namespace.
`,
	Long: `
Get retrieves the permissions on the usage of a name.

The output is a JSON-encoded Permissions object (defined in
v.io/v23/security/access/types.vdl).
`,
}

func runPermissionsGet(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("get: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	perms, _, err := v23.GetNamespace(ctx).GetPermissions(ctx, name)
	if err != nil {
		return err
	}
	return json.NewEncoder(env.Stdout).Encode(perms)
}

var cmdDelete = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runDelete),
	Name:     "delete",
	Short:    "Deletes a name from the namespace",
	ArgsName: "<name>",
	ArgsLong: "<name> is a name to delete.",
	Long:     "Deletes a name from the namespace.",
}

func runDelete(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("delete: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	return v23.GetNamespace(ctx).Delete(ctx, name, flagDeleteSubtree)
}

var cmdRoot = &cmdline.Command{
	Name:  "namespace",
	Short: "resolves and manages names in the Vanadium namespace",
	Long: `
Command namespace resolves and manages names in the Vanadium namespace.

The namespace roots are set from the command line via --v23.namespace.root
command line option or from environment variables that have a name starting
with V23_NAMESPACE, e.g.  V23_NAMESPACE, V23_NAMESPACE_2, V23_NAMESPACE_GOOGLE,
etc.  The command line options override the environment.
`,
	Children: []*cmdline.Command{cmdGlob, cmdMount, cmdUnmount, cmdResolve, cmdResolveToMT, cmdPermissions, cmdDelete},
}
