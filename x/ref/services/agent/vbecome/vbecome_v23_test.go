// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test/v23test"
)

func writeRoledConfig() (path string, shutdown func(), err error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return dir, nil, err
	}
	err = ioutil.WriteFile(filepath.Join(dir, "therole.conf"), []byte(`
{
  "Members": ["root:child"],
  "Extend": true
}
`), 0644)
	return dir, func() { os.RemoveAll(dir) }, err
}

func TestV23BecomeRole(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	vbecome := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/vbecome")
	principal := v23test.BuildGoPkg(sh, "v.io/x/ref/cmd/principal")
	roled := v23test.BuildGoPkg(sh, "v.io/x/ref/services/role/roled")

	sh.StartRootMountTable()

	dir, shutdown, err := writeRoledConfig()
	if err != nil {
		t.Fatalf("Couldn't write roled config: %v", err)
	}
	defer shutdown()

	sh.Cmd(roled, "--v23.tcp.address=127.0.0.1:0", "--config-dir", dir, "--name", "roled").WithCredentials(sh.ForkCredentials("master")).Start()

	output := sh.Cmd(vbecome, "--role=roled/therole", principal, "dump").Stdout()
	want := regexp.MustCompile(`Default Blessings\s+root:master:therole:root:child`)
	if !want.MatchString(output) {
		t.Errorf("Principal didn't have the role blessing:\n %s", output)
	}
}

func TestV23BecomeName(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	vbecome := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/vbecome")
	principal := v23test.BuildGoPkg(sh, "v.io/x/ref/cmd/principal")

	sh.StartRootMountTable()

	output := sh.Cmd(vbecome, "--name=bob", principal, "dump").Stdout()
	want := regexp.MustCompile(`Default Blessings\s+root:child:bob`)
	if !want.MatchString(output) {
		t.Errorf("Principal didn't have the expected blessing:\n %s", output)
	}
}

func TestV23BecomeRoleAndName(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	vbecome := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/vbecome")
	principal := v23test.BuildGoPkg(sh, "v.io/x/ref/cmd/principal")
	roled := v23test.BuildGoPkg(sh, "v.io/x/ref/services/role/roled")

	sh.StartRootMountTable()

	dir, shutdown, err := writeRoledConfig()
	if err != nil {
		t.Fatalf("Couldn't write roled config: %v", err)
	}
	defer shutdown()

	sh.Cmd(roled, "--v23.tcp.address=127.0.0.1:0", "--config-dir", dir, "--name", "roled").WithCredentials(sh.ForkCredentials("master")).Start()

	output := sh.Cmd(vbecome, "--role=roled/therole", "--name=bob", principal, "dump").Stdout()
	want := regexp.MustCompile(`Default Blessings\s+root:child:bob,root:master:therole:root:child`)
	if !want.MatchString(output) {
		t.Errorf("Principal didn't have the role blessing:\n %s", output)
	}
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
