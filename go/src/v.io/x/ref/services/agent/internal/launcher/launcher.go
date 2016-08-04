// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package launcher contains utilities to launch v23agentd.
package launcher

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/agent/internal/constants"
)

const pkgPath = "v.io/x/ref/services/agent/internal/launcher"

var (
	errFilepathAbs = verror.Register(pkgPath+".errAbsFailed", verror.NoRetry, "{1:}{2:} filepath.Abs failed for {3}")
	errCantCreate  = verror.Register(pkgPath+".errCantCreate", verror.NoRetry, "{1:}{2:} failed to create {3}{:_}")
)

// LaunchAgent launches the agent as a separate process and waits for it to
// serve the principal.
func LaunchAgent(credsDir, agentBin string, printCredsEnv bool, flags ...string) error {
	// Use an absolute path to the credentials directory to avoid relying on
	// a specific cwd setting when starting the agent.
	if path, err := filepath.Abs(credsDir); err != nil {
		return verror.New(errFilepathAbs, nil, credsDir, err)
	} else {
		credsDir = path
	}
	// Use an absolute path to the agent to avoid relying on a specific cwd
	// setting when starting the agent.
	agentBin, err := exec.LookPath(agentBin)
	if err != nil {
		return err
	}
	if path, err := filepath.Abs(agentBin); err != nil {
		return verror.New(errFilepathAbs, nil, agentBin, err)
	} else {
		agentBin = path
	}

	agentRead, agentWrite, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %v", err)
	}
	defer agentRead.Close()
	flags = append(flags,
		fmt.Sprintf("--%s=false", constants.DaemonFlag),
		fmt.Sprintf("--%s=%s", constants.CredentialsFlag, credsDir))
	cmd := exec.Command(agentBin, flags...)
	agentDir := constants.AgentDir(credsDir)
	cmd.Dir = agentDir
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		return verror.New(errCantCreate, nil, agentDir, err)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = new(syscall.SysProcAttr)
	cmd.SysProcAttr.Setsid = true
	// The agentRead/Write pipe is used to get notification from the agent
	// when it's ready to serve the principal.
	cmd.ExtraFiles = append(cmd.ExtraFiles, agentWrite)
	cmd.Env = []string{
		fmt.Sprintf("%s=3", constants.EnvAgentParentPipeFD),
		fmt.Sprintf("%s=%s", constants.EnvAgentNoPrintCredsEnv, map[bool]string{true: "", false: "1"}[printCredsEnv]),
		"PATH=" + os.Getenv("PATH"),
	}
	err = cmd.Start()
	agentWrite.Close()
	if err != nil {
		return fmt.Errorf("failed to run %v: %v", cmd.Args, err)
	}
	pid := cmd.Process.Pid
	vlog.Infof("Started agent for credentials %v with PID %d", credsDir, pid)
	// Make sure we don't leave a disfunctional or zombie agent behind upon
	// error.
	cleanUpAgent := func() {
		defer cmd.Wait()
		if err := syscall.Kill(pid, syscall.SIGINT); err == syscall.ESRCH {
			return
		}
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Millisecond)
			if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
				return
			}
		}
		syscall.Kill(pid, syscall.SIGKILL)
	}
	scanner := bufio.NewScanner(agentRead)
	if !scanner.Scan() || scanner.Text() != constants.ServingMsg {
		cleanUpAgent()
		return fmt.Errorf("failed to receive \"%s\" from agent", constants.ServingMsg)
	}
	if err := scanner.Err(); err != nil {
		cleanUpAgent()
		return fmt.Errorf("failed reading status from agent: %v", err)
	}
	return nil
}
