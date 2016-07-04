// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//
// +build linux

package sysinit

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpstart(t *testing.T) {
	oldTemplate := upstartTemplate
	upstartTemplate = `{{.Date}}
{{.Service}}
{{.Description}}
{{.Binary}}
{{.Command}}
`
	oldUpstartDir := upstartDir
	upstartDir, _ = ioutil.TempDir(".", "etc-init")

	defer func() {
		upstartTemplate = oldTemplate
		upstartDir = oldUpstartDir
	}()

	defer os.RemoveAll(upstartDir)
	u := &UpstartService{
		Service:     "tester",
		Description: "my test",
		Binary:      "/bin/echo",
		Command:     []string{"/bin/echo -n foo"},
	}
	if err := u.Install(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	rc, _ := os.Open(upstartDir + "/tester.conf")
	lines := bufio.NewScanner(rc)
	lines.Scan()
	timestr := lines.Text()
	_, err := time.Parse(dateFormat, timestr)
	if err != nil {
		t.Fatalf("unexpected error parsing time: %v, err: %v", t, err)
	}
	lines.Scan()
	if lines.Text() != "tester" {
		t.Fatalf("unexpected output: %s", lines.Text())
	}
	lines.Scan()
	lines.Scan()
	if lines.Text() != "/bin/echo" {
		t.Fatalf("unexpected output: %s", lines.Text())
	}
	lines.Scan()
	if lines.Scan() {
		t.Fatalf("failed to find end of file")
	}
}

func TestSystemd(t *testing.T) {
	s := &SystemdService{
		Service:     "tester",
		Description: "my test",
		Binary:      "/bin/echo",
		Command:     []string{"/bin/echo", "-n", "foo"},
	}

	oldSystemdDir := systemdDir
	oldSystemdTmpFileDir := systemdTmpFileDir
	oldAction := action
	oldFindSystemdSystemCtl := findSystemdSystemCtl

	systemdDir, _ = ioutil.TempDir(".", "usr-lib-systemd-system")
	defer os.RemoveAll(systemdDir)
	systemdTmpFileDir, _ = ioutil.TempDir(".", "usr-lib-tmpfiles.d")
	defer os.RemoveAll(systemdTmpFileDir)

	var cmd, act, srv string
	action = func(command, action, service string) error {
		cmd, act, srv = command, action, service
		return nil
	}

	findSystemdSystemCtl = func() string {
		return "systemctl"
	}

	defer func() {
		systemdDir = oldSystemdDir
		systemdTmpFileDir = oldSystemdTmpFileDir
		action = oldAction
		findSystemdSystemCtl = oldFindSystemdSystemCtl
	}()

	if err := s.Install(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if want, got := "systemctl", cmd; want != got {
		t.Errorf("action command: want %q, got %q", want, got)
	}
	if want, got := "enable", act; want != got {
		t.Errorf("action action: want %q, got %q", want, got)
	}
	if want, got := "tester", srv; want != got {
		t.Errorf("action service: want %q, got %q", want, got)
	}

	c, err := ioutil.ReadFile(filepath.Join(systemdDir, "tester.service"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	contents := string(c)
	if !strings.Contains(contents, "Description=my test") {
		t.Errorf("bad Description in generated service spec: %v", contents)
	}
	if !strings.Contains(contents, "ExecStart=/bin/echo -n foo") {
		t.Errorf("bad ExecStart in generated service spec: %v", contents)
	}
}
