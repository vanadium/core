// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

var (
	// The format of the log file names is:
	// program.host.user.log.logger.tag.YYYYMMDD-HHmmss.pid
	logFileRE = regexp.MustCompile(`^(.*)\.([^.]*)\.([^.]*)\.log\.([^.]*)\.([^.]*)\.(........-......)\.([0-9]*)$`)
)

type logFile struct {
	// TODO(rthellend): Some of these fields are not used by anything yet,
	// but they will be eventually when we need to sort the log files
	// associated with a given instance.
	symlink                          bool
	program, host, user, logger, tag string
	time                             time.Time
	pid                              int
}

func parseFileInfo(dir string, fileInfo os.FileInfo) (*logFile, error) {
	fileName := fileInfo.Name()
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		buf := make([]byte, syscall.NAME_MAX)
		n, err := syscall.Readlink(filepath.Join(dir, fileName), buf)
		if err != nil {
			return nil, err
		}
		linkName := string(buf[:n])
		lf, err := parseFileName(linkName)
		if err != nil {
			return nil, err
		}
		lf.symlink = true
		return lf, nil
	}
	return parseFileName(fileName)
}

func parseFileName(fileName string) (*logFile, error) {
	if m := logFileRE.FindStringSubmatch(fileName); len(m) != 0 {
		t, err := time.ParseInLocation("20060102-150405", m[6], time.Local)
		if err != nil {
			return nil, err
		}
		pid, err := strconv.ParseInt(m[7], 10, 32)
		if err != nil {
			return nil, err
		}
		return &logFile{
			program: m[1],
			host:    m[2],
			user:    m[3],
			logger:  m[4],
			tag:     m[5],
			time:    t,
			pid:     int(pid),
		}, nil
	}
	return nil, errors.New("not a recognized log file name")
}
