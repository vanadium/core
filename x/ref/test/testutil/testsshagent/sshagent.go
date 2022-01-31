// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testsshagent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// SSHAdd runs ssh-add in the specified directory to connect to the
// agent sockname to add the specified keys.
func SSHAdd(dir, sockname string, args ...string) (string, error) {
	cmd := exec.Command("ssh-add", args...)
	cmd.Dir = dir
	cmd.Env = []string{"SSH_AUTH_SOCK=" + sockname}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed %v: to add ssh keys: %v: %s", strings.Join(cmd.Args, " "), err, output)
	}
	return string(output), nil
}

// StartSSHAgent starts an ssh agent and returns a cleanup function and
// the socket name to use for connecting to the agent.
func StartSSHAgent() (func(), string, error) {
	cmd := exec.Command("ssh-agent")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", err
	}
	lines := strings.Split(string(output), "\n")
	first := lines[0]
	addr := strings.TrimPrefix(first, "SSH_AUTH_SOCK=")
	addr = strings.TrimSuffix(addr, "; export SSH_AUTH_SOCK;")
	second := lines[1]
	pidstr := strings.TrimPrefix(second, "SSH_AGENT_PID=")
	pidstr = strings.TrimSuffix(pidstr, "; export SSH_AGENT_PID;")
	pid, err := strconv.ParseInt(pidstr, 10, 64)
	if err != nil {
		return func() {}, "", fmt.Errorf("failed to parse pid from %v", second)
	}
	cleanup := func() {
		syscall.Kill(int(pid), syscall.SIGTERM)
		if testing.Verbose() {
			fmt.Println(string(output))
			fmt.Printf("killing: %v\n", int(pid))
		}
	}
	return cleanup, addr, nil
}

// StartPreconfiguredAgent starts an ssh agent and preconfigures it with
// the supplied keys.
// It returns a cleanup function, the socket name for the agent,
// the directory that the keys are created in and an error.
func StartPreconfiguredAgent(keydir string, keys ...string) (func(), string, error) {
	cleanup, addr, err := StartSSHAgent()
	if err != nil {
		return func() {}, "", err
	}
	for _, k := range keys {
		if err := os.Chmod(filepath.Join(keydir, k), 0600); err != nil {
			panic(err)
		}
	}
	if _, err := SSHAdd(keydir, addr, keys...); err != nil {
		return func() {}, "", err
	}
	list, _ := SSHAdd(keydir, addr, "-l")
	fmt.Printf("%s\n", list)
	return cleanup, addr, nil
}
