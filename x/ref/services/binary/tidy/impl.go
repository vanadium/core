// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"fmt"
	"sort"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/x/lib/cmdline"
	"v.io/x/lib/set"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/internal/binarylib"
	"v.io/x/ref/services/internal/profiles"
	"v.io/x/ref/services/repository"
)

var cmdBinaryTidy = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runBinaryTidy),
	Name:   "binary",
	Short:  "Binary sub-command tidies a specified binaryd",
	Long: `
Binary sub-command removes all binaries from a specified binaryd that
are not referenced by an applicationd envelope stored in the specified
applicationd.
`,
	ArgsName: "<applicationd> <binaryd>",
	ArgsLong: `
<applicationd> is the name or endpoint of the applicationd instance
sourcing the envelopes.
<binaryd> is the name or endpoint of a binaryd instance to clean up.
`,
}

// simpleGlob globs the provided endpoint as the namespace cmd does.
func mapGlob(ctx *context.T, pattern string, mapFunc func(string)) ([]error, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ns := v23.GetNamespace(ctx)
	c, err := ns.Glob(ctx, pattern)
	if err != nil {
		vlog.Infof("ns.Glob(%q) failed: %v", pattern, err)
		return nil, err
	}

	errors := []*naming.GlobError{}
	for res := range c {
		switch v := res.(type) {
		case *naming.GlobReplyEntry:
			if v.Value.Name != "" {
				mapFunc(v.Value.Name)
			}
		case *naming.GlobReplyError:
			errors = append(errors, &v.Value)
		}
	}

	globErrors := make([]error, 0, len(errors))
	for _, err := range errors {
		globErrors = append(globErrors, fmt.Errorf("glob error: %s: %v", err.Name, err.Error))
	}
	return globErrors, nil
}

func logGlobErrors(env *cmdline.Env, errors []error) {
	for _, err := range errors {
		vlog.Errorf("Glob error: %v", err)
	}
}

// getProfileNames uses glob to extract the list of profile names
// available from the binary server specified by endpoint.
func getProfileNames(ctx *context.T, env *cmdline.Env, endpoint string) ([]string, error) {
	profiles, err := profiles.GetKnownProfiles()
	if err != nil {
		return nil, err
	}

	pnames := make([]string, 0, len(profiles))
	for _, p := range profiles {
		pnames = append(pnames, p.Label)
	}
	return pnames, nil
}

func getNames(ctx *context.T, env *cmdline.Env, endpoint string) ([]string, error) {
	resultSet := make(map[string]struct{})
	errors, err := mapGlob(ctx, endpoint, func(s string) {
		resultSet[s] = struct{}{}
	})

	if err != nil {
		return nil, err
	}
	logGlobErrors(env, errors)
	s := set.String.ToSlice(resultSet)
	sort.Strings(s)
	return s, nil
}

func runBinaryTidy(ctx *context.T, env *cmdline.Env, args []string) error { //nolint:gocyclo
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("match: incorrect number of arguments, expected %d, got %d", expected, got)
	}

	appEndpoint, binEndpoint := args[0], args[1]

	profileNames, err := getProfileNames(ctx, env, binEndpoint)
	if err != nil {
		return err
	}

	envelopeNames, err := getNames(ctx, env, naming.Join(appEndpoint, "..."))
	if err != nil {
		return err
	}

	// Add every path in use to a set. Deletion scope is limited to
	// only the binEndpoint.
	bpaths := make(map[string]struct{})
	for _, en := range envelopeNames {
		// convert an envelope name into an envelope.
		ac := repository.ApplicationClient(en)

		for _, p := range profileNames {
			e, err := ac.Match(ctx, []string{p})
			if err != nil {
				// This error case is very noisy.
				vlog.VI(2).Infof("applicationd.Match(%s, %s) failed: %v\n", en, p, err)
				continue
			}

			root, relative := naming.SplitAddressName(e.Binary.File)
			if root == binEndpoint || root == "" {
				bpaths[relative] = struct{}{}
			}
			for _, sf := range e.Packages {
				root, relative := naming.SplitAddressName(sf.File)
				if root == binEndpoint || root == "" {
					bpaths[relative] = struct{}{}
				}
			}
		}
	}

	binaryNames, err := getNames(ctx, env, naming.Join(binEndpoint, "..."))
	if err != nil {
		return err
	}

	deletionCandidates := make([]int, 0, len(binaryNames)-len(envelopeNames))
	for i, bn := range binaryNames {
		_, relative := naming.SplitAddressName(bn)
		if _, ok := bpaths[relative]; ok {
			// relative is mentioned in an envelope.
			continue
		}

		if _, err := binarylib.Stat(ctx, bn); err != nil {
			// This name is not a binary.
			continue
		}
		deletionCandidates = append(deletionCandidates, i)
	}

	for _, i := range deletionCandidates {
		b := binaryNames[i]
		if err := binarylib.Delete(ctx, b); err != nil {
			vlog.Errorf("Couldn't delete binary %s: %v", b, err)
		}
	}

	return nil
}

var cmdApplicationTidy = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runApplicationTidy),
	Name:   "application",
	Short:  "Application sub-command tidies a specified applicationd",
	Long: `
Application sub-command uses the Tidy RPC to clean up outdated
envelopes from the specified appilcationd. Call this before tidying a
binaryd instance for maximum tidiness.
`,
	ArgsName: "<applicationd>",
	ArgsLong: `
<applicationd> is the name or endpoint of the applicationd instance
to tidy.
`,
}

func runApplicationTidy(ctx *context.T, env *cmdline.Env, args []string) error {
	vlog.Infof("runApplicationTidy")
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("match: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	ac := repository.ApplicationClient(args[0])
	return ac.TidyNow(ctx)
}

var cmdRoot = &cmdline.Command{
	Name:  "tidy",
	Short: "Tidy binary repositories",
	Long: `
Tidy tidies the Vanadium repository by removing unused
envelopes and binaries.
`,
	Children: []*cmdline.Command{cmdBinaryTidy, cmdApplicationTidy},
}

func main() {
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}
