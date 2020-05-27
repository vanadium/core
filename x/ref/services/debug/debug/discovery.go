// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
)

var (
	cmdDiscoveryScan = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runDiscoveryScan),
		Name:   "scan",
		Short:  "Scan for advertised v23 services",
		Long: `
Scans for advertised v23 services that match the provided query. The set of
protocols over which services are scanned for depends on the platform on which
this command is being run. As of May 2016, services advertised over mDNS should
be scannable on all platforms but services advertised over BluetoothLE would
be scannable only on OS X.
`,
		ArgsName: "[<query>]",
		ArgsLong: `
<query> scans for services that match the provided query. An empty query causes
all services to be scanned for. See https://godoc.org/v.io/v23/discovery#T for
details.

Sample queries:
  v.InterfaceName = "v.io/i"
  v.InterfaceName = "v.io/i" AND v.Attributes["a"] = "v"
  v.Attributes["a"] = "v1" OR v.Attributes["a"] = "v2"
`,
	}
	cmdDiscovery = &cmdline.Command{
		Name:     "discovery",
		Short:    "Scan and advertise using the v23 discovery API",
		Children: []*cmdline.Command{cmdDiscoveryScan},
	}
)

func runDiscoveryScan(ctx *context.T, env *cmdline.Env, args []string) error { //nolint:gocyclo
	var query string
	switch len(args) {
	case 0:
		query = ""
	case 1:
		query = args[0]
	default:
		return env.UsageErrorf("got %d arguments, expected at most 1", len(args))
	}
	d, err := v23.NewDiscovery(ctx)
	if err != nil {
		return err
	}
	updates, err := d.Scan(ctx, query)
	if err != nil {
		return err
	}
	for u := range updates {
		if u.IsLost() {
			fmt.Fprint(env.Stdout, "LOST:", u.Id(), "\n\n")
			continue
		}
		// Pretty print as multi-line text
		fmt.Fprintln(env.Stdout, "FOUND:")
		ad := u.Advertisement()
		fmt.Fprintln(env.Stdout, "Id:           ", ad.Id)
		fmt.Fprintln(env.Stdout, "InterfaceName:", ad.InterfaceName)
		if len(ad.Addresses) > 0 {
			fmt.Fprintln(env.Stdout, "Addresses:    ", "{")
			for _, a := range ad.Addresses {
				fmt.Fprintln(env.Stdout, " ", a)
			}
			fmt.Fprintln(env.Stdout, "}")
		}
		if len(ad.Attributes) > 0 {
			fmt.Fprintln(env.Stdout, "Attributes:   ", "{")
			var keys []string
			for k := range ad.Attributes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(env.Stdout, "  %q: %q\n", k, ad.Attributes[k])
			}
			fmt.Fprintln(env.Stdout, "}")
		}
		if len(ad.Attachments) > 0 {
			fmt.Fprintln(env.Stdout, "Attachments:   ", "{")
			var keys []string
			for k := range ad.Attachments {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(env.Stdout, "  %q: (%d bytes)\n", k, len(ad.Attachments[k]))
			}
			fmt.Fprintln(env.Stdout, "}")
		}
		fmt.Fprintln(env.Stdout)
	}
	return nil
}
