// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"v.io/x/ref"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/test/expect"
	"v.io/x/ref/test/v23test"
)

func getHostname(t *testing.T) string {
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("Hostname() failed: %v", err)
		return ""
	}
	return hostname
}

func start(c *v23test.Cmd) *expect.Session {
	c.Start()
	return c.S
}

func TestV23Mount(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	nhName := fmt.Sprintf("test-%s-%d", getHostname(t), os.Getpid())
	sh.StartRootMountTableWithOpts(mounttablelib.Opts{NhName: nhName})

	name := sh.Vars[ref.EnvNamespacePrefix]
	clientBin := v23test.BuildGoPkg(sh, "v.io/x/ref/cmd/mounttable")
	clientCreds := sh.ForkCredentials("cmd")

	// Get the neighborhood endpoint from the mounttable.
	neighborhoodEndpoint := start(sh.Cmd(clientBin, "glob", name, "nh").WithCredentials(clientCreds)).ExpectSetEventuallyRE(`^nh (.*) \(Deadline .*\)$`)[0][1]

	sh.Cmd(clientBin, "mount", name+"/myself", name, "5m").WithCredentials(clientCreds).Run()
	sh.Cmd(clientBin, "mount", name+"/google", "/www.google.com:80", "5m").WithCredentials(clientCreds).Run()

	// Test glob output. We expect three entries (two we mounted plus the
	// neighborhood). The 'myself' entry should be the IP:port we
	// specified for the mounttable.
	glob := start(sh.Cmd(clientBin, "glob", name, "*").WithCredentials(clientCreds))
	matches := glob.ExpectSetEventuallyRE(
		`^google /www\.google\.com:80 \(Deadline .*\)$`,
		`^myself (.*) \(Deadline .*\)$`,
		`^nh `+regexp.QuoteMeta(neighborhoodEndpoint)+` \(Deadline .*\)$`)
	if matches[1][1] != name {
		t.Fatalf("expected 'myself' entry to be %q, but was %q", name, matches[1][1])
	}

	// Test globbing on the neighborhood name. Its endpoint should be the
	// endpoint of the mount table.
	glob = start(sh.Cmd(clientBin, "glob", "/"+neighborhoodEndpoint, nhName).WithCredentials(clientCreds))
	matches = glob.ExpectSetEventuallyRE("^" + regexp.QuoteMeta(nhName) + ` (.*) \(Deadline .*\)$`)

	if matches[0][1] != name {
		t.Fatalf("expected endpoint of mount table for name %s", nhName)
	}
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
