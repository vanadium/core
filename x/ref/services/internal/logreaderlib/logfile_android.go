// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build android

// Package logreaderlib implements the LogFile interface from
// v.io/v23/services/logreader for Android applications where log lines are
// written using the Android logging API.
package logreaderlib

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/logreader"
	"v.io/v23/verror"
)

var errNoSizeForPriority = verror.Register(pkgPath+".errNoSizeForPriority", verror.NoRetry, "{1:}{2:} log entries may be present but cannot determine the size for {:}")

// NewLogFileService returns a logreader.LogFileServer suitable for use on
// Android devices where the underlying data is extracted using the "logcat"
// command. See  http://developer.android.com/tools/debugging/debugging-log.html
//
// The first argument to this function is ignored. It is only present to ensure
// that this function has the same signature whether this package is being
// compiled for android or not.
func NewLogFileService(_, suffix string) interface{} {
	return logreader.LogFileServer(androidLogFile(suffix))
}

type androidLogFile string

func (l androidLogFile) Size(ctx *context.T, _ rpc.ServerCall) (int64, error) {
	switch l.priority() {
	case "":
		return 0, nil
	case "V":
		// Verbose means everything, so "logcat -g" is what we want.
	default:
		// Cannot obtain size without running through the whole log and
		// filtering for the appropriate tags. Error out instead of
		// working so hard.
		return 0, verror.New(errNoSizeForPriority, ctx, l)
	}
	// Output from the "adb logcat -g" command I ran in February 2016 is
	// below.  This output format is not part of the SDK and is subject to
	// change at any time.
	//
	// main: ring buffer is 256Kb (255Kb consumed), max entry is 5120b, max payload is 4076b
	// system: ring buffer is 256Kb (252Kb consumed), max entry is 5120b, max payload is 4076b
	// crash: ring buffer is 256Kb (6Kb consumed), max entry is 5120b, max payload is 4076b
	re, err := regexp.Compile("([0-9]+[^ ]+)[bB] consumed")
	if err != nil {
		return 0, err
	}

	cmd := logcat("-g")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	defer cmd.Wait()
	scanner := bufio.NewScanner(out)
	var sum int64
	for scanner.Scan() {
		m := re.FindSubmatch(scanner.Bytes())
		if len(m) > 1 && len(m[1]) > 0 {
			s := string(m[1])
			mult := 1
			switch {
			case strings.HasSuffix(s, "K"):
				s = strings.TrimSuffix(s, "K")
				mult = 1 << 10
			case strings.HasSuffix(s, "M"):
				s = strings.TrimSuffix(s, "M")
				mult = 1 << 20
			}
			n, err := strconv.Atoi(s)
			if err != nil {
				return sum, verror.New(errOperationFailed, ctx, fmt.Errorf("failed to parse output of 'logcat -g': %q (%v)", s, err))
			}
			sum += int64(n * mult)
		}
	}
	return sum, scanner.Err()
}

func (l androidLogFile) ReadLog(ctx *context.T, call logreader.LogFileReadLogServerCall, startpos int64, numEntries int32, follow bool) (int64, error) {
	if len(l) == 0 {
		return 0, verror.NewErrNoExist(ctx)
	}
	priority := "*:" + l.priority()
	var cmd *exec.Cmd
	if follow {
		cmd = logcat(priority)
	} else {
		cmd = logcat("-d", priority)
	}
	// As a cheap form of "auditing", log who is reading the logs!
	readerBlessings, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("ReadLog:Begin: by %v logcat %v, follow:%v", readerBlessings, priority, follow)
	defer ctx.Infof("ReadLog:End:   %v logcat %v, follow:%v", readerBlessings, priority, follow)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	defer out.Close()
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	defer cmd.Process.Kill()
	if startpos > 0 {
		if _, err := io.CopyN(ioutil.Discard, out, startpos); err != nil {
			return 0, err
		}
	}
	reader := newFollowReader(ctx, out, startpos, follow)
	if numEntries == logreader.AllEntries {
		numEntries = int32(math.MaxInt32)
	}
	for i := int32(0); i < numEntries; i++ {
		line, offset, err := reader.readLine()
		if err == io.EOF && i > 0 {
			return reader.tell(), nil
		}
		if err == io.EOF {
			return reader.tell(), verror.NewErrEndOfFile(ctx)
		}
		if err != nil {
			return reader.tell(), verror.New(errOperationFailed, ctx, err)
		}
		if err := call.SendStream().Send(logreader.LogEntry{Position: offset, Line: line}); err != nil {
			return reader.tell(), err
		}
	}
	return reader.tell(), nil
}

// priority returns the substring used by the android logging system.
// Verbose -> V, Info -> I etc.
func (l androidLogFile) priority() string {
	if len(l) == 0 {
		return ""
	}
	return string(l)[0:1]
}

func (l androidLogFile) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, m *glob.Element) error {
	if len(l) > 0 {
		return nil
	}
	// From http://developer.android.com/tools/debugging/debugging-log.html
	sender := call.SendStream()
	for _, v := range []string{"Verbose", "Debug", "Info", "Warning", "Error", "Fatal"} {
		if m.Match(v) {
			sender.Send(naming.GlobChildrenReplyName{Value: v})
		}
	}
	return nil
}

func logcat(args ...string) *exec.Cmd { return exec.Command("/system/bin/logcat", args...) }
