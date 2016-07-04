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

type VcloudVM struct {
	vcloud              string // path to vcloud command
	sshUser             string // ssh into the VM as this user
	projectArg, zoneArg string // common flags used with the vcloud command
	name, ip            string
	workingDir          string
	isDeleted           bool
}

type VcloudVMOptions struct {
	VcloudBinary string // path to the "vcloud" command
}

func newVcloudVM(instanceName string, opt VcloudVMOptions) (vm CloudVM, err error) {
	// TODO: Make sshUser, zone, and project configurable
	g := &VcloudVM{
		vcloud:     opt.VcloudBinary,
		sshUser:    "veyron",
		projectArg: "--project=google.com:veyron",
		zoneArg:    "--zone=us-central1-c",
		isDeleted:  false,
	}

	cmd := exec.Command(g.vcloud, "node", "create", g.projectArg, g.zoneArg, instanceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("setting up new GCE instance (%v) failed. Error: (%v) Output:\n%v", strings.Join(cmd.Args, " "), err, string(output))
	}

	cmd = exec.Command(g.vcloud, "list", g.projectArg, "--noheader", "--fields=EXTERNAL_IP", instanceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("listing instances (%v) failed. Error: (%v) Output:\n%v", strings.Join(cmd.Args, " "), err, string(output))
	}
	tmpIP := strings.TrimSpace(string(output))
	if net.ParseIP(tmpIP) == nil {
		return nil, fmt.Errorf("IP of new instance is not a valid IP address: %v", tmpIP)
	}
	g.ip = tmpIP
	g.name = instanceName

	const workingDir = "/tmp/dmrun"
	output, err = g.RunCommand("mkdir", workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to make working dir: %v (output: %s)", err, output)
	}
	g.workingDir = workingDir

	return g, nil
}

func (g *VcloudVM) Delete() error {
	if g.isDeleted {
		return fmt.Errorf("trying to delete a deleted VcloudVM")
	}
	cmd := g.generateDeleteCmd(false)
	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed deleting GCE instance (%s): %v\nOutput:%v\n", strings.Join(cmd.Args, " "), err, string(output))
	} else {
		g.isDeleted = true
		g.name = ""
		g.ip = ""
	}
	return err
}

func (g *VcloudVM) generateDeleteCmd(forUser bool) *exec.Cmd {
	return exec.Command(g.vcloudCmd(forUser), "node", "delete", g.projectArg, g.zoneArg, g.name)
}

func (g *VcloudVM) Name() string {
	return g.name
}

func (g *VcloudVM) IP() string {
	return g.ip
}

func (g *VcloudVM) RunCommand(args ...string) ([]byte, error) {
	if g.isDeleted {
		return nil, fmt.Errorf("RunCommand called on deleted VcloudVM")
	}

	cmd := g.generateExecCmdForRun(false, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed running [%s] on VM %s", strings.Join(args, " "), g.name)
	}
	return output, err
}

func (g *VcloudVM) RunCommandForUser(args ...string) string {
	if g.isDeleted {
		return ""
	}
	return cmdLine(g.generateExecCmdForRun(true, args...))
}

func cmdLine(cmd *exec.Cmd) string {
	result := cmd.Path
	for _, arg := range cmd.Args[1:] {
		result = fmt.Sprintf("%s %q", result, arg)
	}
	return result
}

func (g *VcloudVM) vcloudCmd(forUser bool) string {
	if forUser {
		// We can't return the vcloud binary that we ran for the steps
		// above, as that one is deleted after use. For now, we assume
		// the user will have a vcloud binary on his path to use.
		return "vcloud"
	}
	return g.vcloud
}

func (g *VcloudVM) generateExecCmdForRun(forUser bool, args ...string) *exec.Cmd {
	return exec.Command(g.vcloudCmd(forUser), append([]string{"sh", g.projectArg, g.name, "cd", g.workingDir, "&&"}, args...)...)
}

func (g *VcloudVM) CopyFile(infile, destination string) error {
	if g.isDeleted {
		return fmt.Errorf("CopyFile called on deleted VcloudVM")
	}

	cmd := exec.Command("gcloud", "compute", g.projectArg, "copy-files", infile, fmt.Sprintf("%s@%s:%s", g.sshUser, g.Name(), path.Join(g.workingDir, destination)), g.zoneArg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed copying %s to %s:%s - %v\nOutput:\n%v", infile, g.name, destination, err, string(output))
	}
	return err
}

func (g *VcloudVM) DeleteCommandForUser() string {
	if g.isDeleted {
		return ""
	}
	return cmdLine(g.generateDeleteCmd(true))
}
