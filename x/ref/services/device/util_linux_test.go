// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package device_test

import (
	"os/user"
	"testing"

	"v.io/x/ref/test/v23test"
)

const psFlags = "-eouser:20,pid"

func makeTestAccounts(t *testing.T, sh *v23test.Shell) {
	sudo := "/usr/bin/sudo"

	if _, err := user.Lookup("vana"); err != nil {
		sh.Cmd(sudo, "/usr/sbin/adduser", "--no-create-home", "vana").Run()
	}

	if _, err := user.Lookup("devicemanager"); err != nil {
		sh.Cmd(sudo, "/usr/sbin/adduser", "--no-create-home", "devicemanager").Run()
	}
}

const runTestOnThisPlatform = true
