// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
)

type AWSVMOptions struct {
	Region       string // region that the AWS VM will be in
	ImageID      string // note that images are specific to a region
	LoginUser    string // depends on the image being used
	InstanceType string // what VM hw configuration
	KeyDir       string // where to store the ssh key on local fs
	Dbg          DebugPrinter
	AWSBinary    string // paths to the respective commands
	SSHBinary    string
	SCPBinary    string
}

type AWSVM struct {
	isDeleted                            bool
	name                                 string
	keyFile                              string    // location of ssh key for VM
	runner                               awsRunner // used to execute "aws" commandline tools
	keyName, securityGroupId, instanceId string    // AWS names for various resources we create
	sshvm                                CloudVM
	dbg                                  DebugPrinter
}

func newAWSVM(instanceName string, opt AWSVMOptions) (vm CloudVM, err error) {
	if opt.Region != "" && opt.ImageID == "" {
		return nil, fmt.Errorf("image ids are region-specific, so if you specify a region, please specify an image id as well")
	}

	setDefault := func(value *string, defaultValue string) {
		if *value == "" {
			*value = defaultValue
		}
	}
	setDefault(&opt.Region, "us-east-1")
	setDefault(&opt.ImageID, "ami-d05e75b8") // "Ubuntu Server 14.04 LTS (HVM), SSD Volume Type"
	setDefault(&opt.LoginUser, "ubuntu")
	setDefault(&opt.InstanceType, "t2.micro")
	setDefault(&opt.KeyDir, "/tmp/")
	setDefault(&opt.AWSBinary, "aws")
	setDefault(&opt.SSHBinary, "ssh")
	setDefault(&opt.SCPBinary, "scp")

	g := &AWSVM{
		isDeleted: false,
		name:      instanceName,
		dbg:       opt.Dbg,
		runner:    newAWSRunner(opt.AWSBinary, opt.Region, instanceName, opt.Dbg),
	}

	if _, err := g.runner.ec2DescribeInstances(); err != nil {
		return nil, fmt.Errorf("unable to describe instances -- are AWS tools set up?: %v", err)
	}

	// Attempt to do a cleanup if we exit this function with a partially-created VM
	defer func() {
		if g.sshvm == nil {
			fmt.Fprintf(os.Stderr, "Cleaning up partially-created AWS VM ...\n")
			g.Delete()
		}
	}()

	if g.keyFile, g.keyName, err = g.runner.ec2CreateKeyPair(opt.KeyDir); err != nil {
		return nil, fmt.Errorf("failed creating key pair: %v", err)
	}
	if g.securityGroupId, err = g.runner.ec2CreateSecurityGroup("security group for dmrun VM " + instanceName); err != nil {
		return nil, fmt.Errorf("failed creating security group for %s: %v", instanceName, err)
	}
	if g.instanceId, err = g.runner.ec2RunInstances(opt.InstanceType, opt.ImageID, g.keyName, g.securityGroupId); err != nil {
		return nil, fmt.Errorf("failed running instance for %s: %v", instanceName, err)
	}

	// Wait until the instance is running and has a public IP address available
	var instance *awsInstance
	if instance, err = g.waitForState("running", 100); err != nil {
		return nil, fmt.Errorf("error waiting for VM %s to be running: %v", g.name, err)
	}
	if instance.PublicIPAddress == "" {
		return nil, fmt.Errorf("VM %s is running but has no public ip?\nState:\n%+v\n", instance)
	}
	g.dbg.Printf("VM %s has ip address %s\n", g.name, instance.PublicIPAddress)

	sshOpt := SSHVMOptions{
		SSHHostIP:  instance.PublicIPAddress,
		SSHUser:    opt.LoginUser,
		SSHOptions: []string{"-i", g.keyFile, "-o", "StrictHostKeyChecking=no", "-o", "ConnectionAttempts=60"},
		SSHBinary:  opt.SSHBinary,
		SCPBinary:  opt.SCPBinary,
		Dbg:        g.dbg,
	}
	if g.sshvm, err = newSSHVM(instanceName, sshOpt); err != nil {
		g.dbg.Printf("Error creating SSH VM: %v\n", err)
		return nil, fmt.Errorf("failed creating sshVM for %s: %v", instanceName, err)
	}

	return g, nil
}

func (g *AWSVM) Delete() error {
	if g.isDeleted {
		return fmt.Errorf("trying to delete a deleted AWSVM")
	}

	g.name = ""
	g.isDeleted = true

	errMsg := make([]string, 0, 10)

	// We may be deleting a partially created object, so we test for each stage of
	// creation (in reverse order of the steps in newAWSVM) and proceed appropriately
	switch {
	case g.sshvm != nil:
		if err := g.sshvm.Delete(); err != nil {
			errMsg = append(errMsg, fmt.Sprint("while deleting ssh VM: [%v]", err))
		}
		g.sshvm = nil
		fallthrough
	case g.instanceId != "":
		if err := g.runner.ec2TerminateInstances(g.instanceId); err != nil {
			errMsg = append(errMsg, fmt.Sprint("while terminating AWS VM: [%v]", err))
		}
		if _, err := g.waitForState("terminated", 100); err != nil {
			errMsg = append(errMsg, fmt.Sprint("while waiting for instance termination: [%v]", err))
		}
		g.instanceId = ""
		fallthrough
	case g.securityGroupId != "":
		if err := g.runner.ec2DeleteSecurityGroup(g.securityGroupId); err != nil {
			errMsg = append(errMsg, fmt.Sprint("while deleting security group: [%v]", err))
		}
		g.securityGroupId = ""
		fallthrough
	case g.keyName != "":
		if err := g.runner.ec2DeleteKeyPair(g.keyName); err != nil {
			errMsg = append(errMsg, fmt.Sprint("while deleting key pair: [%v]", err))
		}
		g.keyName = ""
	}
	if len(errMsg) > 0 {
		return fmt.Errorf(strings.Join(errMsg, ", "))
	}

	return nil
}

func (g *AWSVM) Name() string {
	return g.name
}

func (g *AWSVM) IP() string {
	return g.sshvm.IP()
}

func (g *AWSVM) RunCommand(args ...string) ([]byte, error) {
	if g.isDeleted {
		return nil, fmt.Errorf("RunCommand called on deleted AWSVM")
	}

	return g.sshvm.RunCommand(args...)
}

func (g *AWSVM) RunCommandForUser(args ...string) string {
	return g.sshvm.RunCommandForUser(args...)
}

func (g *AWSVM) CopyFile(infile, destination string) error {
	if g.isDeleted {
		return fmt.Errorf("CopyFile called on deleted AWSVM")
	}

	return g.sshvm.CopyFile(infile, destination)
}

func (g *AWSVM) DeleteCommandForUser() string {
	return strings.Join(
		[]string{
			g.runner.ec2TerminateInstancesCommand(g.instanceId),
			g.runner.ec2CmdString("wait", "instance-terminated", "--instance-ids", g.instanceId),
			g.runner.ec2DeleteSecurityGroupCommand(g.securityGroupId),
			g.runner.ec2DeleteKeyPairCommand(g.keyName),
		}, "; ")
}

// waitForState waits up to the specified # of retries for the VM to be in the specified state,
// sleeping 2 seconds between attempts. Returns the awsInstance response from the last poll.
func (g *AWSVM) waitForState(state string, retries int) (*awsInstance, error) {
	for count := 1; count <= retries; count++ {
		time.Sleep(2 * time.Second)
		if instances, err := g.runner.ec2DescribeInstances(g.instanceId); err != nil {
			return nil, fmt.Errorf("failed to find instance waiting for %s: %v", g.name, err)
		} else if instances[0].State.Name == state {
			return &instances[0], nil
		}
	}
	return nil, fmt.Errorf("timed out waiting for %s to be in state '%s'\n", g.name, state)
}

// awsRunner is used for executing commands via the "aws" commandline tool. Where needed, it
// parses JSON output to get the results of commands, as the text output is not usually
// amenable to parsing.
type awsRunner struct {
	awsBinary     string
	region        string
	namePrefix    string
	dbg           DebugPrinter
	ec2CommonArgs []string
}

func newAWSRunner(binary, region, namePrefix string, dbg DebugPrinter) awsRunner {
	return awsRunner{
		awsBinary:     binary,
		region:        region,
		namePrefix:    namePrefix,
		dbg:           dbg,
		ec2CommonArgs: []string{"ec2", "--output", "json", "--region", region},
	}
}

// ec2Cmd runs an "aws ec2" commandline command and parses the json result. Pass in nil for
// resultMsg when no output is expected.
func (a *awsRunner) ec2Cmd(resultMsg interface{}, args ...string) error {
	cmd := exec.Command(a.awsBinary, append(a.ec2CommonArgs, args...)...)
	errBuf := new(bytes.Buffer)
	cmd.Stderr = errBuf

	output, err := cmd.Output()
	if err != nil || (len(output) == 0 && resultMsg != nil) {
		cmdLine := fmt.Sprint(append([]string{cmd.Path}, cmd.Args[1:]...))
		return fmt.Errorf("failed running [%s]: %v\nOutput:\n%v\nStderr: %v\n", cmdLine, err, output, errBuf.String())
	}

	// no output is okay as long as that's what the caller expected (by providing a nil resultMsg)
	if len(output) == 0 {
		return nil
	}

	err = json.Unmarshal([]byte(output), resultMsg)
	if err != nil {
		cmdLine := fmt.Sprint(append([]string{cmd.Path}, cmd.Args[1:]...))
		return fmt.Errorf("running [%s], failed to unmarshall:\n%v\n(error:%v)", cmdLine, string(output), err)
	}

	return nil
}

// ec2CmdString produces a printable ec2 command that we can give to the user to run by hand
func (a *awsRunner) ec2CmdString(args ...string) string {
	return fmt.Sprintf("%s %s %s", a.awsBinary, strings.Join(a.ec2CommonArgs, " "), strings.Join(args, " "))
}

type awsInstance struct {
	InstanceId     string `json:InstanceId`
	ImageId        string `json:ImageId`
	KeyName        string `json:KeyName`
	InstanceType   string `json:InstanceType`
	SecurityGroups []struct {
		GroupName string `json:GroupName`
		GroupId   string `json:GroupId`
	} `json:SecurityGroups`
	PublicIPAddress  string `json:PublicIpAddress`
	PrivateIPAddress string `json:PrivateIpAddress`
	State            struct {
		Name string `json:Name`
	} `json:State`
}

func (a *awsRunner) ec2DescribeInstances(instanceIds ...string) (instances []awsInstance, err error) {
	var result struct {
		Reservations []struct {
			Instances []awsInstance `json:Instances`
		} `json:Reservations`
	}

	args := []string{"describe-instances"}
	if len(instanceIds) > 0 {
		args = append(args, append([]string{"--instance-ids"}, instanceIds...)...)
	}

	if err = a.ec2Cmd(&result, args...); err != nil {
		err = fmt.Errorf("failed in call to describe-instances: %v", err)
		return
	}

	// There is a reservation for every launch request, and each launch request may launch
	// one or more instances. Collect all the instances over all reservations into the result.
	for _, r := range result.Reservations {
		instances = append(instances, r.Instances...)
	}

	// Make sure we got back as many instances as we expected
	if len(instanceIds) > 0 && len(instances) != len(instanceIds) {
		err = fmt.Errorf("got back an unexpected number of instances: Expected: [%v], Got: [%+v]", instanceIds, instances)
		return
	}

	a.dbg.Printf("EC2: describe-instances found: %+v\n", instances)
	return
}

func (a *awsRunner) ec2CreateKeyPair(keyDir string) (keyFile, KeyName string, err error) {
	var result struct {
		KeyMaterial string `json:KeyMaterial`
		KeyName     string `json:KeyName`
	}

	if err = a.ec2Cmd(&result, "create-key-pair", "--key-name", "dmrun-"+a.namePrefix+"-ssh-key"); err != nil {
		err = fmt.Errorf("failed in create-key-pair: %v", err)
		return "", "", err
	}

	var f *os.File
	if f, err = ioutil.TempFile(keyDir, a.namePrefix); err != nil {
		err = fmt.Errorf("opening tmp file for key: %v", err)
		return
	}
	if err = f.Chmod(0600); err != nil {
		err = fmt.Errorf("chmod(0600) of file %s: %v", f.Name(), err)
		os.Remove(f.Name())
		return
	}
	if _, err = f.WriteString(result.KeyMaterial); err != nil {
		err = fmt.Errorf("writing key to file: %v", err)
		os.Remove(f.Name())
		return
	}
	if err = f.Close(); err != nil {
		err = fmt.Errorf("closing key file: %v", err)
		os.Remove(f.Name())
		return
	}
	a.dbg.Print("EC2: stored ssh key", result.KeyName, "in", f.Name())
	return f.Name(), result.KeyName, nil
}

func (a *awsRunner) ec2CreateSecurityGroup(description string) (securityGroupId string, err error) {
	var result struct {
		GroupId string `json: GroupId`
	}

	if err = a.ec2Cmd(&result, "create-security-group",
		"--group-name", a.namePrefix+"-security-group",
		"--description", "dmrun "+a.namePrefix); err != nil {
		err = fmt.Errorf("failed in create-security-group: %v", err)
		return
	}
	a.dbg.Print("EC2: Created security group", result.GroupId)

	// Set acls on the group. If needed in future, it may make sense to break this out into a
	// separate method

	// authorize-security-group-ingress returns no output
	if err = a.ec2Cmd(nil, "authorize-security-group-ingress",
		"--group-id", result.GroupId, "--protocol", "tcp", "--port", "22",
		"--cidr", "0.0.0.0/0"); err != nil {
		err = fmt.Errorf("failed to authorize ssh ingress: %v", err)
		return
	}
	a.dbg.Print("EC2: Authorized ssh ingress for group", result.GroupId)

	if err = a.ec2Cmd(nil, "authorize-security-group-ingress",
		"--group-id", result.GroupId, "--protocol", "tcp", "--port", "8100-8200",
		"--cidr", "0.0.0.0/0"); err != nil {
		err = fmt.Errorf("failed to authorize port 8100-8200 ingress: %v", err)
		return
	}
	a.dbg.Print("EC2: Authorized ports 8100-8200 ingress for group", result.GroupId)

	return result.GroupId, nil
}

// ec2RunInstances is named after the AWS command it executes. Note that it actually creates the
// instance before running it.
func (a *awsRunner) ec2RunInstances(instanceType, imageId, keyName, securityGroupId string) (instanceId string, err error) {
	var result struct {
		Instances []awsInstance `json:Instances`
	}

	if err = a.ec2Cmd(&result, "run-instances", "--count", "1", "--instance-type", instanceType, "--image-id", imageId, "--key-name", keyName, "--security-group-ids", securityGroupId); err != nil {
		err = fmt.Errorf("failed in run-instances: %v", err)
		return
	}

	if len(result.Instances) == 0 {
		err = fmt.Errorf("got no instances back although run-instances succeeded!")
		return
	} else if len(result.Instances) > 1 {
		err = fmt.Errorf("got more than one running instance in run-instances: %v", result.Instances)
		return
	}

	a.dbg.Printf("EC2: Instance running: %v\n", result.Instances[0])
	return result.Instances[0].InstanceId, nil
}

func (a *awsRunner) ec2TerminateInstances(instanceIds ...string) error {
	if len(instanceIds) == 0 {
		return nil
	}

	var result struct{} // We'll ignore the json result from the aws command
	args := []string{"terminate-instances"}
	args = append(args, append([]string{"--instance-ids"}, instanceIds...)...)

	if err := a.ec2Cmd(&result, args...); err != nil {
		return fmt.Errorf("failed terminating %v: %v", instanceIds, err)
	}
	return nil
}

func (a *awsRunner) ec2TerminateInstancesCommand(instanceIds ...string) string {
	return a.ec2CmdString("terminate-instances") + " --instance-ids  " + strings.Join(instanceIds, " ")
}

func (a *awsRunner) ec2DeleteSecurityGroup(securityGroupId string) error {
	var result struct{} // There's no json result from this
	if err := a.ec2Cmd(&result, "delete-security-group", "--group-id", securityGroupId); err != nil {
		return fmt.Errorf("failed deleting security group %s: %v", securityGroupId, err)
	}
	return nil
}

func (a *awsRunner) ec2DeleteSecurityGroupCommand(securityGroupId string) string {
	return a.ec2CmdString("delete-security-group") + " --group-id  " + securityGroupId
}

func (a *awsRunner) ec2DeleteKeyPair(keyName string) error {
	var result struct{} // There's no json result from this
	if err := a.ec2Cmd(&result, "delete-key-pair", "--key-name", keyName); err != nil {
		return fmt.Errorf("failed deleting key pair %s: %v", keyName, err)
	}
	return nil
}

func (a *awsRunner) ec2DeleteKeyPairCommand(keyName string) string {
	return a.ec2CmdString("delete-key-pair") + " --key-name  " + keyName
}
