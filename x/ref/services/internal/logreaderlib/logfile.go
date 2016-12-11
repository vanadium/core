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
		return "", verror.New(errOperationFailed, nil, name)
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
			return 0, verror.New(verror.ErrNoExist, ctx, fname)
		}
		ctx.Errorf("Stat(%v) failed: %v", fname, err)
		return 0, verror.New(errOperationFailed, ctx, fname)
	}
	if fi.IsDir() {
		return 0, verror.New(errOperationFailed, ctx, fname)
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
			return 0, verror.New(verror.ErrNoExist, ctx, fname)
		}
		return 0, verror.New(errOperationFailed, ctx, fname)
	}
	if startpos, err = f.Seek(startpos, 0); err != nil {
		return 0, verror.New(errOperationFailed, ctx, fname, err)
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
			return reader.tell(), verror.NewErrEndOfFile(ctx)
		}
		if err != nil {
			return reader.tell(), verror.New(errOperationFailed, ctx, fname)
		}
		if err := call.SendStream().Send(logreader.LogEntry{Position: offset, Line: line}); err != nil {
			return reader.tell(), err
		}
	}
	return reader.tell(), nil
}

// GlobChildren__ returns the list of files in a directory on a stream.
// The list is empty if the object is a file.
func (i *logfileService) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, m *glob.Element) error {
	ctx.VI(1).Infof("%v.GlobChildren__()", i.suffix)
	dirName, err := translateNameToFilename(i.root, i.suffix)
	if err != nil {
		return err
	}
	stat, err := os.Stat(dirName)
	if err != nil {
		if os.IsNotExist(err) {
			return verror.New(verror.ErrNoExist, ctx, dirName)
		}
		return verror.New(errOperationFailed, ctx, dirName)
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
				call.SendStream().Send(naming.GlobChildrenReplyName{Value: name})
			}
		}
	}
	return nil
}
