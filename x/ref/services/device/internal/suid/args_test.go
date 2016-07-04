// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suid

import (
	"flag"
	"reflect"
	"testing"

	"v.io/v23/verror"
)

// Note: The specific user chosen has no consequence other than it has the same
// ids across the set of systems we are testing.
const (
	testUserName = "daemon"
	testUid      = 1
	testGid      = 1
)

func TestParseArguments(t *testing.T) {
	cases := []struct {
		cmdline  []string
		env      []string
		errID    verror.ID
		expected WorkParameters
	}{

		{
			[]string{"setuidhelper"},
			[]string{},
			errUserNameMissing.ID,
			WorkParameters{},
		},

		{
			[]string{"setuidhelper", "--minuid", "1", "--username", testUserName},
			[]string{"A=B"},
			"",
			WorkParameters{
				uid:       testUid,
				gid:       testGid,
				workspace: "",
				agentsock: "",
				logDir:    "",
				argv0:     "",
				argv:      []string{"unnamed_app"},
				envv:      []string{"A=B"},
				dryrun:    false,
				remove:    false,
				chown:     false,
				kill:      false,
				killPids:  nil,
			},
		},

		{
			[]string{"setuidhelper", "--minuid", "1", "--username", testUserName, "--workspace", "/hello",
				"--logdir", "/logging", "--agentsock", "/tmp/sXXXX", "--run", "/bin/v23", "--", "one", "two"},
			[]string{"A=B"},
			"",
			WorkParameters{
				uid:       testUid,
				gid:       testGid,
				workspace: "/hello",
				agentsock: "/tmp/sXXXX",
				logDir:    "/logging",
				argv0:     "/bin/v23",
				argv:      []string{"unnamed_app", "one", "two"},
				envv:      []string{"A=B"},
				dryrun:    false,
				remove:    false,
				chown:     false,
				kill:      false,
				killPids:  nil,
			},
		},

		{
			[]string{"setuidhelper", "--username", testUserName},
			[]string{"A=B"},
			errUIDTooLow.ID,
			WorkParameters{},
		},

		{
			[]string{"setuidhelper", "--rm", "hello", "vanadium"},
			[]string{"A=B"},
			"",
			WorkParameters{
				uid:       0,
				gid:       0,
				workspace: "",
				agentsock: "",
				logDir:    "",
				argv0:     "",
				argv:      []string{"hello", "vanadium"},
				envv:      nil,
				dryrun:    false,
				remove:    true,
				chown:     false,
				kill:      false,
				killPids:  nil,
			},
		},

		{
			[]string{"setuidhelper", "--chown", "--username", testUserName, "--dryrun", "--minuid", "1", "/tmp/foo", "/tmp/bar"},
			[]string{"A=B"},
			"",
			WorkParameters{
				uid:       -1,
				gid:       -1,
				workspace: "",
				agentsock: "",
				logDir:    "",
				argv0:     "",
				argv:      []string{"/tmp/foo", "/tmp/bar"},
				envv:      nil,
				dryrun:    true,
				remove:    false,
				chown:     true,
				kill:      false,
				killPids:  nil,
			},
		},

		{
			[]string{"setuidhelper", "--kill", "235", "451"},
			[]string{"A=B"},
			"",
			WorkParameters{
				uid:       0,
				gid:       0,
				workspace: "",
				agentsock: "",
				logDir:    "",
				argv0:     "",
				argv:      nil,
				envv:      nil,
				dryrun:    false,
				remove:    false,
				chown:     false,
				kill:      true,
				killPids:  []int{235, 451},
			},
		},

		{
			[]string{"setuidhelper", "--kill", "235", "451oops"},
			[]string{"A=B"},
			errAtoiFailed.ID,
			WorkParameters{
				uid:       0,
				gid:       0,
				workspace: "",
				agentsock: "",
				logDir:    "",
				argv0:     "",
				argv:      nil,
				envv:      nil,
				dryrun:    false,
				remove:    false,
				chown:     false,
				kill:      true,
				killPids:  nil,
			},
		},

		{
			[]string{"setuidhelper", "--minuid", "1", "--username", testUserName, "--workspace", "/hello", "--progname", "binaryd/vanadium/app/testapp",
				"--logdir", "/logging", "--agentsock", "/tmp/2981298123/s", "--run", "/bin/v23", "--dryrun", "--", "one", "two"},
			[]string{"A=B"},
			"",
			WorkParameters{
				uid:       -1,
				gid:       -1,
				workspace: "/hello",
				agentsock: "/tmp/2981298123/s",
				logDir:    "/logging",
				argv0:     "/bin/v23",
				argv:      []string{"binaryd/vanadium/app/testapp", "one", "two"},
				envv:      []string{"A=B"},
				dryrun:    true,
				remove:    false,
				chown:     false,
				kill:      false,
				killPids:  nil,
			},
		},
	}

	for _, c := range cases {
		var wp WorkParameters
		fs := flag.NewFlagSet(c.cmdline[0], flag.ExitOnError)
		setupFlags(fs)
		fs.Parse(c.cmdline[1:])
		if err := wp.ProcessArguments(fs, c.env); (err != nil || c.errID != "") && verror.ErrorID(err) != c.errID {
			t.Fatalf("got %s (%v), expected %q error", verror.ErrorID(err), err, c.errID)
		}
		if !reflect.DeepEqual(wp, c.expected) {
			t.Fatalf("got %#v expected %#v", wp, c.expected)
		}
	}
}
