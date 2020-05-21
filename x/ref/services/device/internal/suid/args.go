// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suid

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"v.io/v23/verror"
)

const pkgPath = "v.io/x/ref/services/device/internal/suid"

var (
	errUserNameMissing = verror.Register(pkgPath+".errUserNameMissing", verror.NoRetry, "{1:}{2:} --username missing{:_}")
	errUnknownUser     = verror.Register(pkgPath+".errUnknownUser", verror.NoRetry, "{1:}{2:} --username {3}: unknown user{:_}")
	errInvalidUID      = verror.Register(pkgPath+".errInvalidUID", verror.NoRetry, "{1:}{2:} user.Lookup() returned an invalid uid {3}{:_}")
	errInvalidGID      = verror.Register(pkgPath+".errInvalidGID", verror.NoRetry, "{1:}{2:} user.Lookup() returned an invalid gid {3}{:_}")
	errUIDTooLow       = verror.Register(pkgPath+".errUIDTooLow", verror.NoRetry, "{1:}{2:} suidhelper uid {3} is not permitted because it is less than {4}{:_}")
	errAtoiFailed      = verror.Register(pkgPath+".errAtoiFailed", verror.NoRetry, "{1:}{2:} strconv.Atoi({3}) failed{:_}")
	errInvalidFlags    = verror.Register(pkgPath+".errInvalidFlags", verror.NoRetry, "{1:}{2:} invalid flags ({3} are set){:_}")
)

type WorkParameters struct {
	uid       int
	gid       int
	workspace string
	agentsock string
	logDir    string
	argv0     string
	argv      []string
	envv      []string
	dryrun    bool
	remove    bool
	chown     bool
	kill      bool
	killPids  []int
}

type ArgsSavedForTest struct {
	Uname    string
	Workpace string
	Run      string
	LogDir   string
}

const SavedArgs = "V23_SAVED_ARGS"

var (
	flagUsername, flagWorkspace, flagLogDir, flagRun, flagProgName, flagAgentSock *string
	flagMinimumUid                                                                *int64
	flagRemove, flagKill, flagChown, flagDryrun                                   *bool
)

func init() {
	setupFlags(flag.CommandLine)
}

func setupFlags(fs *flag.FlagSet) {
	const uidThreshold = 501
	flagUsername = fs.String("username", "", "The UNIX user name used for the other functions of this tool.")
	flagWorkspace = fs.String("workspace", "", "Path to the application's workspace directory.")
	flagAgentSock = fs.String("agentsock", "", "Path to the application's security agent socket.")
	flagLogDir = fs.String("logdir", "", "Path to the log directory.")
	flagRun = fs.String("run", "", "Path to the application to exec.")
	flagProgName = fs.String("progname", "unnamed_app", "Visible name of the application, used in argv[0]")
	flagMinimumUid = fs.Int64("minuid", uidThreshold, "UIDs cannot be less than this number.")
	flagRemove = fs.Bool("rm", false, "Remove the file trees given as command-line arguments.")
	flagKill = fs.Bool("kill", false, "Kill process ids given as command-line arguments.")
	flagChown = fs.Bool("chown", false, "Change owner of files and directories given as command-line arguments to the user specified by this flag")
	flagDryrun = fs.Bool("dryrun", false, "Elides root-requiring systemcalls.")
}

func cleanEnv(env []string) []string {
	nenv := []string{}
	for _, e := range env {
		if !strings.HasPrefix(e, "V23_SUIDHELPER_TEST") {
			nenv = append(nenv, e)
		}
	}
	return nenv
}

// checkFlagCombinations makes sure that a valid combination of flags has been
// specified for rm/kill/chown
//
// --rm and --kill are modal. Complain if any other flag is set along with one of
// those.  --chown allows specification of --username, --dryrun, and --minuid,
// but nothing else
func checkFlagCombinations(fs *flag.FlagSet) error {
	if !(*flagRemove || *flagKill || *flagChown) {
		return nil
	}

	// Count flags that are set. The device manager test always sets --minuid=1
	// and --test.run=TestSuidHelper so when in a test, tolerate those.
	flagsToIgnore := map[string]string{}
	if os.Getenv("V23_SUIDHELPER_TEST") != "" {
		flagsToIgnore["minuid"] = "1"
		flagsToIgnore["test.run"] = "TestSuidHelper"
	}
	if *flagChown {
		// Allow all values of --username, --dryrun, and --minuid
		flagsToIgnore["username"] = "*"
		flagsToIgnore["dryrun"] = "*"
		flagsToIgnore["minuid"] = "*"
	}

	counter := 0
	fs.Visit(func(f *flag.Flag) {
		if flagsToIgnore[f.Name] != f.Value.String() && flagsToIgnore[f.Name] != "*" {
			counter++
		}
	})

	if counter > 1 {
		return verror.New(errInvalidFlags, nil, counter, "--rm and --kill cannot be used with any other flag. --chown can only be used with --username, --dryrun, and --minuid")
	}
	return nil
}

// warnMissingSuidPrivs makes it a little easier to debug when suid privs are required but
// are not present. It's not a comprehensive check -- e.g. we may be running as user
// <username> and suppress the warning, but still fail to chown a file owned by some other user.
func warnMissingSuidPrivs(uid int) {
	osUid, osEuid := os.Getuid(), os.Geteuid()
	if osUid == 0 || osEuid == 0 || osUid == uid || osEuid == uid {
		return
	}

	fmt.Fprintln(os.Stderr, "uid is ", os.Getuid(), ", effective uid is ", os.Geteuid())
	fmt.Fprintln(os.Stderr, "WARNING: suidhelper is not root. Is your filesystem mounted with nosuid?")
}

// ParseArguments populates the WorkParameter object from the provided args
// and env strings.
func (wp *WorkParameters) ProcessArguments(fs *flag.FlagSet, env []string) error {
	if err := checkFlagCombinations(fs); err != nil {
		return err
	}

	if *flagRemove {
		wp.remove = true
		wp.argv = fs.Args()
		return nil
	}

	if *flagKill {
		wp.kill = true
		for _, p := range fs.Args() {
			pid, err := strconv.Atoi(p)
			if err != nil {
				wp.killPids = nil
				return verror.New(errAtoiFailed, nil, p, err)
			}
			wp.killPids = append(wp.killPids, pid)
		}
		return nil
	}

	if *flagDryrun {
		wp.uid, wp.gid = -1, -1
	} else {
		username := *flagUsername
		if username == "" {
			return verror.New(errUserNameMissing, nil)
		}

		usr, err := user.Lookup(username)
		if err != nil {
			return verror.New(errUnknownUser, nil, username)
		}

		uid, err := strconv.ParseInt(usr.Uid, 0, 32)
		if err != nil {
			return verror.New(errInvalidUID, nil, usr.Uid)
		}
		gid, err := strconv.ParseInt(usr.Gid, 0, 32)
		if err != nil {
			return verror.New(errInvalidGID, nil, usr.Gid)
		}
		warnMissingSuidPrivs(int(uid))

		// Uids less than 501 can be special so we forbid running as them.
		if uid < *flagMinimumUid {
			return verror.New(errUIDTooLow, nil,
				uid, *flagMinimumUid)
		}
		wp.uid = int(uid)
		wp.gid = int(gid)
	}
	wp.dryrun = *flagDryrun

	// At this point, all flags allowed by --chown have been processed
	if *flagChown {
		wp.chown = true
		wp.argv = fs.Args()
		return nil
	}

	// Preserve the arguments for examination by the test harness if executed
	// in the course of a test.
	if os.Getenv("V23_SUIDHELPER_TEST") != "" {
		env = cleanEnv(env)
		b := new(bytes.Buffer)
		enc := json.NewEncoder(b)
		// nolint: errcheck
		enc.Encode(ArgsSavedForTest{
			Uname:    *flagUsername,
			Workpace: *flagWorkspace,
			Run:      *flagRun,
			LogDir:   *flagLogDir,
		})
		env = append(env, SavedArgs+"="+b.String())
		wp.dryrun = true
	}

	wp.workspace = *flagWorkspace
	wp.agentsock = *flagAgentSock
	wp.argv0 = *flagRun
	wp.logDir = *flagLogDir
	wp.argv = append([]string{*flagProgName}, fs.Args()...)
	// TODO(rjkroege): Reduce the environment to the absolute minimum needed.
	wp.envv = env

	return nil
}
