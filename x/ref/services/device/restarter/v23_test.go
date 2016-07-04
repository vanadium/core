// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"
	"text/template"

	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/test/v23test"
)

func TestV23RestartExitCode(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	var (
		scriptDir = sh.MakeTempDir()
		counter   = filepath.Join(scriptDir, "counter")
		script    = filepath.Join(scriptDir, "test.sh")
		restarter = v23test.BuildGoPkg(sh, "v.io/x/ref/services/device/restarter")
	)
	if err := writeScript(
		script,
		`#!/bin/bash
# Increment the contents of the counter file
readonly COUNT=$(expr $(<"{{.Counter}}") + 1)
echo -n $COUNT >{{.Counter}}
# Exit code is 0 if the counter is less than 5
[[ $COUNT -lt 5 ]]; exit $?
`,
		struct{ Counter string }{
			Counter: counter,
		}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		RestartExitCode string
		WantError       string
		WantCounter     string
	}{
		{
			// With --restart-exit-code=0, the script should be kicked off
			// 5 times till the counter reaches 5 and the last iteration of
			// the script will exit.
			RestartExitCode: "0",
			WantError:       "exit status 1",
			WantCounter:     "5",
		},
		{
			// With --restart-exit-code=!0, the script will be executed only once.
			RestartExitCode: "!0",
			WantError:       "",
			WantCounter:     "1",
		},
		{
			// --restart-exit-code=!1, should be the same
			// as --restart-exit-code=0 for this
			// particular script only exits with 0 or 1
			RestartExitCode: "!1",
			WantError:       "exit status 1",
			WantCounter:     "5",
		},
	}
	for _, test := range tests {
		// Clear out the counter file.
		if err := ioutil.WriteFile(counter, []byte("0"), 0644); err != nil {
			t.Fatalf("%q: %v", counter, err)
		}
		// Run the script under the restarter.
		cmd := sh.Cmd(restarter, "--restart-exit-code="+test.RestartExitCode, "bash", "-c", script)
		cmd.ExitErrorIsOk = true
		cmd.Run()
		var gotError string
		if cmd.Err != nil {
			gotError = cmd.Err.Error()
		}
		if got, want := gotError, test.WantError; got != want {
			t.Errorf("%+v: Got %q, want %q", test, got, want)
		}
		if buf, err := ioutil.ReadFile(counter); err != nil {
			t.Errorf("ioutil.ReadFile(%q) failed: %v", counter, err)
		} else if got, want := string(buf), test.WantCounter; got != want {
			t.Errorf("%+v: Got %q, want %q", test, got, want)
		}
	}
}

func writeScript(dstfile, tmpl string, args interface{}) error {
	t, err := template.New(dstfile).Parse(tmpl)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, args); err != nil {
		return err
	}
	if err := ioutil.WriteFile(dstfile, buf.Bytes(), 0700); err != nil {
		return err
	}
	return nil
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
