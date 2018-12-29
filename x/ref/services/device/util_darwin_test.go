// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package device_test

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"
	"testing"

	"v.io/x/ref/test/v23test"
)

const runTestOnThisPlatform = true
const psFlags = "-ej"

type uidMap map[int]struct{}

func (uids uidMap) findAvailable() (int, error) {
	// Accounts starting at 501 are available. Don't use the largest
	// UID because on a corporate imaged Mac, this will overlap with
	// another employee's UID. Instead, use the first available UID >= 501.
	for newuid := 501; newuid < 1e6; newuid++ {
		if _, ok := uids[newuid]; !ok {
			uids[newuid] = struct{}{}
			return newuid, nil
		}
	}
	return 0, fmt.Errorf("Couldn't find an available UID")
}

func newUidMap(sh *v23test.Shell) uidMap {
	// `dscl . -list /Users UniqueID` into a datastructure.
	userstring := sh.Cmd("dscl", ".", "-list", "/Users", "UniqueID").Stdout()
	users := strings.Split(userstring, "\n")

	uids := make(map[int]struct{}, len(users))
	for _, line := range users {
		fields := re.Split(line, -1)
		if len(fields) > 1 {
			if uid, err := strconv.Atoi(fields[1]); err == nil {
				uids[uid] = struct{}{}
			}
		}
	}
	return uids
}

func makeAccount(sh *v23test.Shell, uid int, uname, fullname string) {
	sudo := "/usr/bin/sudo"
	args := []string{"dscl", ".", "-create", "/Users/" + uname}

	run := func(extraArgs ...string) {
		sh.Cmd(sudo, append(args, extraArgs...)...).Run()
	}

	run()
	run("UserShell", "/bin/bash")
	run("RealName", fullname)
	run("UniqueID", strconv.FormatInt(int64(uid), 10))
	run("PrimaryGroupID", "20")
}

func makeTestAccounts(t *testing.T, sh *v23test.Shell) {
	_, needVanaErr := user.Lookup("vana")
	_, needDevErr := user.Lookup("devicemanager")

	if needVanaErr == nil && needDevErr == nil {
		return
	}

	uids := newUidMap(sh)
	if needVanaErr != nil {
		vanauid, err := uids.findAvailable()
		if err != nil {
			t.Fatalf("Can't make test accounts: %v", err)
		}
		makeAccount(sh, vanauid, "vana", "Vanadium White")
	}
	if needDevErr != nil {
		devmgruid, err := uids.findAvailable()
		if err != nil {
			t.Fatalf("Can't make test accounts: %v", err)
		}
		makeAccount(sh, devmgruid, "devicemanager", "Devicemanager")
	}
}
