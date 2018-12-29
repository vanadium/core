// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux darwin

package suid

import (
	"encoding/binary"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"v.io/v23/verror"
)

var (
	errChownFailed        = verror.Register(pkgPath+".errChownFailed", verror.NoRetry, "{1:}{2:} os.Chown({3}, {4}, {5}) failed{:_}")
	errGetwdFailed        = verror.Register(pkgPath+".errGetwdFailed", verror.NoRetry, "{1:}{2:} os.Getwd failed{:_}")
	errStartProcessFailed = verror.Register(pkgPath+".errStartProcessFailed", verror.NoRetry, "{1:}{2:} syscall.StartProcess({3}) failed{:_}")
	errRemoveAllFailed    = verror.Register(pkgPath+".errRemoveAllFailed", verror.NoRetry, "{1:}{2:} os.RemoveAll({3}) failed{:_}")
	errFindProcessFailed  = verror.Register(pkgPath+".errFindProcessFailed", verror.NoRetry, "{1:}{2:} os.FindProcess({3}) failed{:_}")
	errKillFailed         = verror.Register(pkgPath+".errKillFailed", verror.NoRetry, "{1:}{2:} os.Process.Kill({3}) failed{:_}")
)

// Chown is only availabe on UNIX platforms so this file has a build
// restriction.
func (hw *WorkParameters) Chown() error {
	chown := func(path string, _ os.FileInfo, inerr error) error {
		if inerr != nil {
			return inerr
		}
		if hw.dryrun {
			log.Printf("[dryrun] os.Chown(%s, %d, %d)", path, hw.uid, hw.gid)
			return nil
		}
		return os.Chown(path, hw.uid, hw.gid)
	}

	chownPaths := hw.argv
	if !hw.chown {
		// Chown was invoked as part of regular suid execution, rather than directly
		// via --chown. In that case, we chown the workspace, log directory, and,
		// if specified, the agent socket path
		// TODO(rjkroege): Ensure that the device manager can read log entries.
		chownPaths = []string{hw.workspace, hw.logDir}
		if hw.agentsock != "" {
			chownPaths = append(chownPaths, hw.agentsock)
		}
	}

	for _, p := range chownPaths {
		if err := filepath.Walk(p, chown); err != nil {
			return verror.New(errChownFailed, nil, p, hw.uid, hw.gid, err)
		}
	}
	return nil
}

func (hw *WorkParameters) Exec() error {
	attr := new(syscall.ProcAttr)

	dir, err := os.Getwd()
	if err != nil {
		log.Printf("error Getwd(): %v", err)
		return verror.New(errGetwdFailed, nil, err)
	}
	attr.Dir = dir
	attr.Env = hw.envv
	attr.Files = []uintptr{
		uintptr(syscall.Stdin),
		uintptr(syscall.Stdout),
		uintptr(syscall.Stderr),
	}

	attr.Sys = new(syscall.SysProcAttr)
	attr.Sys.Setsid = true
	if hw.dryrun {
		log.Printf("[dryrun] syscall.Setgid(%d)", hw.gid)
		log.Printf("[dryrun] syscall.Setuid(%d)", hw.uid)
	} else if syscall.Getuid() != hw.uid || syscall.Getgid() != hw.gid {
		attr.Sys.Credential = new(syscall.Credential)
		attr.Sys.Credential.Gid = uint32(hw.gid)
		attr.Sys.Credential.Uid = uint32(hw.uid)
	}

	// Make sure the child won't talk on the fd we use to talk back to the parent
	syscall.CloseOnExec(PipeToParentFD)

	// Start the child process
	pid, _, err := syscall.StartProcess(hw.argv0, hw.argv, attr)
	if err != nil {
		if !hw.dryrun {
			log.Printf("StartProcess failed: argv: %q %#v attr: %#v, attr.Sys: %#v, attr.Sys.Cred: %#v error: %v", hw.argv0, hw.argv, attr, attr.Sys, attr.Sys.Credential, err)
		} else {
			log.Printf("StartProcess failed: %v", err)
		}
		return verror.New(errStartProcessFailed, nil, hw.argv0, err)
	}

	// Return the pid of the new child process
	pipeToParent := os.NewFile(PipeToParentFD, "pipe_to_parent_wr")
	if err = binary.Write(pipeToParent, binary.LittleEndian, int32(pid)); err != nil {
		log.Printf("Problem returning pid to parent: %v", err)
	} else {
		log.Printf("Returned pid %v to parent", pid)
	}

	os.Exit(0)
	return nil // Not reached.
}

func (hw *WorkParameters) Remove() error {
	for _, p := range hw.argv {
		if err := os.RemoveAll(p); err != nil {
			return verror.New(errRemoveAllFailed, nil, p, err)
		}
	}
	return nil
}

func (hw *WorkParameters) Kill() error {
	for _, pid := range hw.killPids {

		switch err := syscall.Kill(pid, 9); err {
		case syscall.ESRCH:
			// No such PID.
			log.Printf("process pid %d already killed", pid)
		case nil:
			log.Printf("process pid %d killed successfully", pid)
		default:
			// Something went wrong.
			return verror.New(errKillFailed, nil, pid, err)
		}

	}
	return nil
}
