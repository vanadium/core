// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"fmt"
	"net"
	"os/exec"
	"path"
	"strings"
)

type SSHVM struct {
	ssh, scp      string   // paths to the respective commands.
	sshArgs       []string // flags for ssh/scp
	sshUserAtHost string   // target of ssh
	ip            string
	workingDir    string
	isDeleted     bool
	dbg           DebugPrinter
}

type SSHVMOptions struct {
	SSHHostIP  string   // ip of the machine to ssh to
	SSHUser    string   // username to use
	SSHOptions []string // flags to ssh command. Use this to indicate the key file, for example
	SSHBinary  string   // path to the "ssh" command
	SCPBinary  string   // path to the "scp" command
	Dbg        DebugPrinter
}

func newSSHVM(instanceName string, opt SSHVMOptions) (vm CloudVM, err error) {
	// Make sure we got a valid ip
	tmpIP := strings.TrimSpace(opt.SSHHostIP)
	if net.ParseIP(tmpIP) == nil {
		return nil, fmt.Errorf("IP of new ssh instance is not a valid IP address: %v", tmpIP)
	}

	g := &SSHVM{
		ssh:           "ssh",
		scp:           "scp",
		sshArgs:       opt.SSHOptions,
		ip:            tmpIP,
		sshUserAtHost: tmpIP,
		isDeleted:     false,
		dbg:           opt.Dbg,
	}

	if opt.SSHBinary != "" {
		g.ssh = opt.SSHBinary
	}
	if opt.SCPBinary != "" {
		g.scp = opt.SCPBinary
	}
	if opt.SSHUser != "" {
		g.sshUserAtHost = fmt.Sprint(opt.SSHUser, "@", g.ip)
	}

	const workingDir = "/tmp/dmrun"
	if output, err := g.RunCommand("exit"); err != nil {
		return nil, fmt.Errorf("SSH failed: %v", string(output))
	}
	if _, err := g.RunCommand("test", "!", "-e", workingDir); err != nil {
		return nil, fmt.Errorf("working dir %v already exists on target. Please clean up any previous dmrun instance.", workingDir)
	}
	output, err := g.RunCommand("mkdir", workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to make working dir: %v (output: %s)", err, output)
	}
	g.workingDir = workingDir

	return g, nil
}

func (g *SSHVM) Delete() error {
	if g.isDeleted {
		return fmt.Errorf("trying to delete a deleted SSHVM")
	}

	oldWorkingDir := g.workingDir
	g.workingDir = ""
	if output, err := g.RunCommand("rm", "-rf", oldWorkingDir); err != nil {
		return fmt.Errorf("deleting working dir: %v (output: %v)", err, output)
	}

	g.isDeleted = true
	g.ip = ""
	return nil
}

func (g *SSHVM) Name() string {
	return g.ip
}

func (g *SSHVM) IP() string {
	return g.ip
}

func (g *SSHVM) RunCommand(args ...string) ([]byte, error) {
	if g.isDeleted {
		return nil, fmt.Errorf("RunCommand called on deleted SSHVM")
	}

	cmd := g.generateExecCmdForRun(args...)

	g.dbg.Printf("SSHVM: %s %v\n", cmd.Path, cmd.Args[1:])

	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed running [%s] on ssh VM %s", strings.Join(args, " "), g.Name())
	}
	return output, err
}

func (g *SSHVM) RunCommandForUser(args ...string) string {
	if g.isDeleted {
		return ""
	}
	cmd := g.generateExecCmdForRun(args...)

	result := cmd.Path
	for i := 1; i < len(cmd.Args); i++ {
		result = fmt.Sprintf("%s %q", result, cmd.Args[i])
	}
	return result
}

func (g *SSHVM) generateExecCmdForRun(args ...string) *exec.Cmd {
	return exec.Command(g.ssh, append(append(g.sshArgs, g.sshUserAtHost, "cd", g.workingDir, "&&"), args...)...)
}

func (g *SSHVM) CopyFile(infile, destination string) error {
	if g.isDeleted {
		return fmt.Errorf("CopyFile called on deleted SSHVM")
	}

	target := fmt.Sprint(g.sshUserAtHost, ":", path.Join(g.workingDir, destination))
	cmd := exec.Command(g.scp, append(g.sshArgs, infile, target)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed copying %s to %s:%s - %v\nOutput:\n%v", infile, g.Name(), destination, err, string(output))
	}
	return err
}

func (g *SSHVM) DeleteCommandForUser() string {
	return g.RunCommandForUser("rm", "-rf", g.workingDir)
}
