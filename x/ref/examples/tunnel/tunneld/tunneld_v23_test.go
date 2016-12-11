// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"v.io/x/ref"
	"v.io/x/ref/test/v23test"
)

func TestV23Tunneld(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	tunneldBin := v23test.BuildGoPkg(sh, "v.io/x/ref/examples/tunnel/tunneld")
	vsh := v23test.BuildGoPkg(sh, "v.io/x/ref/examples/tunnel/vsh")
	mounttableBin := v23test.BuildGoPkg(sh, "v.io/x/ref/cmd/mounttable")

	// Start tunneld with a known endpoint.
	cmd := sh.Cmd(tunneldBin, "--v23.tcp.address=127.0.0.1:0", "--name=tunnel/test")
	cmd.Start()
	tunnelEndpoint := cmd.S.ExpectVar("NAME")

	// Run remote command with the endpoint.
	if want, got := "HELLO ENDPOINT\n", sh.Cmd(vsh, tunnelEndpoint, "echo", "HELLO", "ENDPOINT").Stdout(); want != got {
		t.Fatalf("unexpected output, got %q, want %q", got, want)
	}

	if want, got := "HELLO NAME\n", sh.Cmd(vsh, "tunnel/test", "echo", "HELLO", "NAME").Stdout(); want != got {
		t.Fatalf("unexpected output, got %q, want %q", got, want)
	}

	// Send input to remote command.
	want := "HELLO SERVER"
	cmd = sh.Cmd(vsh, tunnelEndpoint, "cat")
	cmd.SetStdinReader(strings.NewReader(want))
	if got := cmd.Stdout(); want != got {
		t.Fatalf("unexpected output, got %s, want %s", got, want)
	}

	// And again with a file redirection this time.
	outDir := sh.MakeTempDir()
	outPath := filepath.Join(outDir, "hello.txt")

	cmd = sh.Cmd(vsh, tunnelEndpoint, "cat > "+outPath)
	cmd.SetStdinReader(strings.NewReader(want))
	cmd.Run()
	if got, err := ioutil.ReadFile(outPath); err != nil {
		t.Fatalf("ReadFile(%v) failed: %v", outPath, err)
	} else if string(got) != want {
		t.Fatalf("unexpected output, got %s, want %s", string(got), want)
	}

	// Verify that all published names are there.
	root := sh.Vars[ref.EnvNamespacePrefix]

	cmd = sh.Cmd(mounttableBin, "glob", root, "tunnel/test")
	cmd.Start()

	// Expect one entry: the tunnel name.
	matches := cmd.S.ExpectSetEventuallyRE("tunnel/test" + " (.*) \\(Deadline .*\\)")

	// The full endpoint should be the one we saw originally.
	if got, want := matches[0][1], tunnelEndpoint; "/"+got != want {
		t.Fatalf("expected tunnel endpoint %s to be %s, but it was not", got, want)
	}
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
