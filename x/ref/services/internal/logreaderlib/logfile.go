// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !android

// Package logreaderlib implements the LogFile interface from
// v.io/v23/services/logreader, which can be used to allow remote access to log
// files, and the ChildrenGlobber interface from v.io/v23/rpc to find
// the files in a logs directory.
package logreaderlib

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/services/logreader"
	"v.io/v23/verror"
)

// NewLogFileService returns a new log file server.
func NewLogFileService(root, suffix string) interface{} {
	return logreader.LogFileServer(&logfileService{filepath.Clean(root), suffix})
}

// translateNameToFilename returns the file name that corresponds to the object
// name.
func translateNameToFilename(root, name string) (string, error) {
	name = filepath.Join(strings.Split(name, "/")...)
	p := filepath.Join(root, name)
	// Make sure we're not asked to read a file outside of the root
	// directory. This could happen if suffix contains "../", which get
	// collapsed by filepath.Join().
	if !strings.HasPrefix(p, root) {
		return "", fmt.Errorf("%v: outside of root directory", name)
	}
	return p, nil
}

// logfileService holds the state of a logfile invocation.
type logfileService struct {
	// root is the root directory from which the object names are based.
	root string
	// suffix is the suffix of the current invocation that is assumed to
	// be used as a relative object name to identify a log file.
	suffix string
}

// Size returns the size of the log file, in bytes.
func (i *logfileService) Size(ctx *context.T, _ rpc.ServerCall) (int64, error) {
	ctx.VI(1).Infof("%v.Size()", i.suffix)
	fname, err := translateNameToFilename(i.root, i.suffix)
	if err != nil {
		return 0, err
	}
	fi, err := os.Stat(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, verror.ErrNoExist.Errorf(ctx, "does not exist: %v", fname)
		}
		ctx.Errorf("Stat(%v) failed: %v", fname, err)
		return 0, fmt.Errorf("failed to stat %v: %v", fname, err)
	}
	if fi.IsDir() {
		return 0, fmt.Errorf("%v is a directory", fname)
	}
	return fi.Size(), nil
}

// ReadLog returns log entries from the log file.
func (i *logfileService) ReadLog(ctx *context.T, call logreader.LogFileReadLogServerCall, startpos int64, numEntries int32, follow bool) (int64, error) {
	ctx.VI(1).Infof("%v.ReadLog(%v, %v, %v)", i.suffix, startpos, numEntries, follow)
	fname, err := translateNameToFilename(i.root, i.suffix)
	if err != nil {
		return 0, err
	}
	f, err := os.Open(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, verror.ErrNoExist.Errorf(ctx, "does not exist: %v", fname)
		}
		return 0, fmt.Errorf("failed to open: %v: %v", fname, err)
	}
	if startpos, err = f.Seek(startpos, 0); err != nil {
		return 0, fmt.Errorf("failed to seek to start: %v: %v", fname, err)
	}
	reader := newFollowReader(ctx, f, startpos, follow)
	if numEntries == logreader.AllEntries {
		numEntries = int32(math.MaxInt32)
	}
	for n := int32(0); n < numEntries; n++ {
		line, offset, err := reader.readLine()
		if err == io.EOF && n > 0 {
			return reader.tell(), nil
		}
		if err == io.EOF {
			return reader.tell(), verror.ErrorfEndOfFile(ctx, "end of file")
		}
		if err != nil {
			return reader.tell(), fmt.Errorf("failed to read line: %v: %v", fname, err)
		}
		if err := call.SendStream().Send(logreader.LogEntry{Position: offset, Line: line}); err != nil {
			return reader.tell(), err
		}
	}
	return reader.tell(), nil
}

// GlobChildren__ returns the list of files in a directory on a stream.
// The list is empty if the object is a file.
//nolint:golint // API change required.
func (i *logfileService) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, m *glob.Element) error {
	ctx.VI(1).Infof("%v.GlobChildren__()", i.suffix)
	dirName, err := translateNameToFilename(i.root, i.suffix)
	if err != nil {
		return err
	}
	stat, err := os.Stat(dirName)
	if err != nil {
		if os.IsNotExist(err) {
			return verror.ErrNoExist.Errorf(ctx, "does not exist: %v", dirName)
		}
		return fmt.Errorf("failed to stat %v: %v", dirName, err)
	}
	if !stat.IsDir() {
		return nil
	}

	f, err := os.Open(dirName)
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		fi, err := f.Readdir(100)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		for _, file := range fi {
			name := file.Name()
			if m.Match(name) {
				//nolint:errcheck
				call.SendStream().Send(naming.GlobChildrenReplyName{Value: name})
			}
		}
	}
	return nil
}
