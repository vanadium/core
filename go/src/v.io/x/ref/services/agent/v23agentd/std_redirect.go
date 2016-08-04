// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	timeFormat          = "20060102.150405.000000"
	stdoutSuffix        = ".stdout"
	stderrSuffix        = ".stderr"
	AgentOutFileGCLimit = 20
)

func outFileNames() (stdout, stderr string) {
	prefix := time.Now().Format(timeFormat)
	stdout = prefix + stdoutSuffix
	stderr = prefix + stderrSuffix
	return
}

func isOutFileName(name string) bool {
	var suffix string
	switch {
	case strings.HasSuffix(name, stdoutSuffix):
		suffix = stdoutSuffix
	case strings.HasSuffix(name, stderrSuffix):
		suffix = stderrSuffix
	default:
		return false
	}
	name = strings.TrimSuffix(name, suffix)
	_, err := time.Parse(timeFormat, name)
	return err == nil
}

func gcOutFiles(dir string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	var outFiles []string
	for _, f := range files {
		if isOutFileName(f.Name()) {
			outFiles = append(outFiles, f.Name())
		}
	}
	if len(outFiles) <= AgentOutFileGCLimit {
		return nil
	}
	for _, f := range outFiles[:len(outFiles)-AgentOutFileGCLimit] {
		if err := os.Remove(filepath.Join(dir, f)); err != nil {
			return err
		}
	}
	return nil
}

// redirectOutput sends stdout and stderr to timestamped log files.  It also
// takes care of garbage-collecting older log files.
func redirectOutput(logDir string) error {
	if err := gcOutFiles(logDir); err != nil {
		return err
	}
	redirect := func(f *os.File, fileName string) error {
		const flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
		file, err := os.OpenFile(filepath.Join(logDir, fileName), flags, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		return syscall.Dup2(int(file.Fd()), int(f.Fd()))
	}
	stdout, stderr := outFileNames()
	if err := redirect(os.Stdout, stdout); err != nil {
		return fmt.Errorf("failed to direct stdout: %v", err)
	}
	if err := redirect(os.Stderr, stderr); err != nil {
		return fmt.Errorf("failed to direct stderr: %v", err)
	}
	return nil
}

func detachStdInOutErr(agentDir string) error {
	logsDir := filepath.Join(agentDir, "logs")
	if err := os.MkdirAll(logsDir, 0700); err != nil {
		return err
	}
	if err := redirectOutput(logsDir); err != nil {
		return err
	}
	return os.Stdin.Close()
}
