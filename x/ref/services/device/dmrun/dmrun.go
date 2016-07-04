// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command dmrun runs a binary on a remote VM instance using device manager.
//
// dmrun creates the VM instance, installs and starts device manager on it, and
// then installs and starts an app from the specified binary.
//
// dmrun uses the credentials it is running with in order to claim the device
// manager and provide the app with blessings.  To specify credentials for
// dmrun, use the V23_CREDENTIALS environment variable instead of the
// --v23.credentials flag.
//
// Usage:
//   dmrun [ENV=VAL ...] path/to/binary [--flag=val ...]
//
// All flags and environment variable settings are passed to the app.
//
// Usage examples:
//   vcloud backend -- vcloud must be properly set up to talk to GCE:
//       dmrun ${JIRI_ROOT}/release/go/bin/principal dump
//
//   aws backend -- "aws" command must be installed and access+secret keys configured:
//       dmrun --aws ${JIRI_ROOT}/release/go/bin/principal dump
//
//       (For help setting up aws, see
//          https://docs.google.com/document/d/1hRo6if4IF4EMKJNFu8HB1kbnk-D7UMe47iJuPH0CRAg/view )
//
//   ssh backend can also be invoked directly to talk to an existing VM:
//        (e.g. AWS ubuntu VM set up so you can ssh with "ssh -i /tmp/key" w/o typing a password.)
//        dmrun --ssh "ubuntu@54.152.51.129" --sshoptions "-i /tmp/key" ${JIRI_ROOT}/release/go/bin/principal dump
//

package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"v.io/jiri"
	"v.io/jiri/project"
	"v.io/x/lib/cmdline"
	"v.io/x/ref"
	"v.io/x/ref/services/device/dmrun/backend"
)

var (
	workDir        string
	vcloud         string
	device         string
	vm             backend.CloudVM
	cleanupOnDeath func()
)

var dmBins = [...]string{
	"v.io/x/ref/services/device/deviced",
	"v.io/x/ref/services/device/restarter",
	"v.io/x/ref/services/agent/v23agentd",
	"v.io/x/ref/services/device/inithelper",
	"v.io/x/ref/services/device/suidhelper",
}

const (
	vcloudBin     = "v.io/x/devtools/vcloud"
	deviceBin     = "v.io/x/ref/services/device/device"
	devicexRemote = "https://vanadium.googlesource.com/release.go.x.ref"
	devicex       = "services/device/devicex"
)

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
	if cleanupOnDeath != nil {
		savedCleanupFn := cleanupOnDeath
		cleanupOnDeath = func() {
			fmt.Fprintf(os.Stderr, "Avoided recursive call to cleanup in die()\n")
		}
		savedCleanupFn()
	}
	os.Exit(1)
}

func dieIfErr(err error, format string, args ...interface{}) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error: %v\n", err)
		die(format, args...)
	}
}

// getPath returns the filesystem path to a file specified by a remote and a
// repository-relative path.  If multiple projects have the same remote, the
// function will die with an error.
func getPath(remote, file string) string {
	env := cmdline.EnvFromOS()
	jirix, err := jiri.NewX(env)
	dieIfErr(err, "jiri.NewX(%v) failed: %v", env, err)
	projects, err := project.LocalProjects(jirix, project.FastScan)
	dieIfErr(err, "project.LocalProjects(%v, %v) failed: %v", jirix, project.FastScan, err)
	path := ""
	for _, p := range projects {
		if p.Remote == remote {
			if path != "" {
				die("Projects at paths %q and %q both have remote %s", path, p.Path, remote)
			}
			path = p.Path
		}
	}
	if path == "" {
		die("Couldn't find project with remote %s", remote)
	}
	return filepath.Join(path, filepath.FromSlash(file))
}

// setupWorkDir creates a directory for all the local files created by this
// tool.
func setupWorkDir() {
	var err error
	workDir, err = ioutil.TempDir("", filepath.Base(os.Args[0]))
	dieIfErr(err, "Couldn't set up work dir")
	dieIfErr(os.Chmod(workDir, 0777), "Couldn't chmod work dir")
	fmt.Printf("Working dir: %s\n", workDir)
}

// buildV23Binaries builds the specified binaries and returns the paths to the
// executables.
func buildV23Binaries(pkg ...string) []string {
	fmt.Print("Building ", pkg, " ...")
	defer fmt.Println("Done.")
	args := append([]string{"go", "install", "-x"}, pkg...)
	cmd := exec.Command("jiri", args...)
	cmd.Env = append(os.Environ(), "GOBIN="+workDir)
	output, err := cmd.CombinedOutput()
	dieIfErr(err, "Running build command %v failed. Output:\n%v", strings.Join(cmd.Args, " "), string(output))
	dest := make([]string, len(pkg))
	for i, p := range pkg {
		dest[i] = filepath.Join(workDir, path.Base(p))
	}
	return dest
}

// createArchive creates a zip archive from the given files.
func createArchive(files []string) string {
	zipFile := filepath.Join(workDir, "dm.zip")
	z, err := os.OpenFile(zipFile, os.O_CREATE|os.O_WRONLY, os.FileMode(0644))
	dieIfErr(err, "Couldn't create zip archive file")
	defer z.Close()
	w := zip.NewWriter(z)
	for _, file := range files {
		info, err := os.Stat(file)
		dieIfErr(err, "Couldn't stat %v", file)
		fh, err := zip.FileInfoHeader(info)
		dieIfErr(err, "Couldn't set up file info header")
		fh.Method = zip.Deflate
		fwrite, err := w.CreateHeader(fh)
		dieIfErr(err, "Couldn't create writer")
		fread, err := os.Open(file)
		dieIfErr(err, "Couldn't creater reader")
		_, err = io.Copy(fwrite, fread)
		dieIfErr(err, "Couldn't write to archive")
		dieIfErr(fread.Close(), "Couldn't close reader")
	}
	dieIfErr(w.Close(), "Couldn't close zip archive")
	return zipFile
}

// setupInstance creates a new VM instance and returns its name and IP address.
func setupInstance(vmOptions interface{}) (backend.CloudVM, string, string) {
	fmt.Println("Setting up instance ...")
	currUser, err := user.Current()
	dieIfErr(err, "Couldn't obtain current user")
	instanceName := fmt.Sprintf("%s-%s", currUser.Username, time.Now().UTC().Format("20060102-150405"))
	vm, err = backend.CreateCloudVM(instanceName, vmOptions)
	dieIfErr(err, "VM Instance Creation Failed: %v", err)

	// Make sure nothing is using the ports we plan to give to deviced.
	// TODO(caprita): Don't hardcode the ports and all that.
	if output, err := vm.RunCommand("!", "netstat", "-tulpn", "2>/dev/null", "|", "grep", "'LISTEN'", "|", "grep", "-E", "':8150 |:8160 |:8151 '"); err != nil {
		die("device manager ports are already in use:\n" + string(output))
	}

	instanceIP := vm.IP()
	// Install unzip so we can unpack the archive.
	// TODO(caprita): Use tar instead.
	output, err := vm.RunCommand("sudo", "apt-get", "install", "unzip")
	dieIfErr(err, "Installing unzip failed. Output:\n%v", string(output))
	fmt.Println("Created VM instance", instanceName, "with IP", instanceIP)
	return vm, instanceName, instanceIP
}

// installArchive ships the archive to the VM instance and unpacks it.
func installArchive(archive, instance string) {
	err := vm.CopyFile(archive, "")
	dieIfErr(err, "Copying archive failed: %v", err)
	output, err := vm.RunCommand("unzip", path.Join("./", filepath.Base(archive)), "-d", "./unpacked")
	dieIfErr(err, "Extracting archive failed. Output:\n%v", string(output))
}

// installDevice installs and starts device manager, and returns the public key
// and pairing token needed for claiming.
func installDevice(instance string) (string, string) {
	fmt.Println("Installing device manager...")
	defer fmt.Println("Done installing device manager.")
	output, err := vm.RunCommand("V23_DEVICE_DIR=`pwd`/dm", "./unpacked/devicex", "install", "./unpacked", "--single_user", "--", "--v23.tcp.address=:8151", "--deviced-port=8150", "--use-pairing-token")
	dieIfErr(err, "Installing device manager failed. Output:\n%v", string(output))
	output, err = vm.RunCommand("V23_DEVICE_DIR=`pwd`/dm", "./unpacked/devicex", "start")
	dieIfErr(err, "Starting device manager failed. Output:\n%v", string(output))
	// Grab the token and public key from the device manager log.
	dieAfter := time.After(5 * time.Second)
	firstIteration := true
	for {
		if !firstIteration {
			select {
			case <-dieAfter:
				die("Failed to find token and public key in log: %v", string(output))
			case <-time.After(100 * time.Millisecond):
			}
		} else {
			firstIteration = false
		}
		output, err = vm.RunCommand("cat", "./dm/dmroot/device-manager/logs/deviced.INFO")
		dieIfErr(err, "Reading device manager log failed. Output:\n%v", string(output))
		pairingTokenRE := regexp.MustCompile("Device manager pairing token: (.*)")
		matches := pairingTokenRE.FindSubmatch(output)
		if matches == nil {
			continue
		}
		pairingToken := string(matches[1])
		publicKeyRE := regexp.MustCompile("public_key: (.*)")
		matches = publicKeyRE.FindSubmatch(output)
		if matches == nil {
			continue
		}
		publicKey := string(matches[1])
		return publicKey, pairingToken
	}
}

// setCredentialsEnv sets the command's environment to share the principal of
// dmrun.
func setCredentialsEnv(cmd *exec.Cmd) {
	// TODO(caprita): This doesn't work with --v23.credentials.
	if creds := os.Getenv(ref.EnvCredentials); len(creds) > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", ref.EnvCredentials, creds))
	} else if agentCreds := os.Getenv(ref.EnvAgentPath); len(agentCreds) > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", ref.EnvAgentPath, agentCreds))
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: no credentials found. You'll probably have authorization issues later on.\n")
	}
}

// claimDevice claims the device manager, blessing it with extension.
func claimDevice(deviceName, ip, publicKey, pairingToken, extension string) {
	fmt.Println("claiming device manager ...")
	cmd := exec.Command(device, "claim", deviceName, extension, pairingToken, publicKey)
	setCredentialsEnv(cmd)
	output, err := cmd.CombinedOutput()
	dieIfErr(err, "Claiming device manager (%v) failed. Output:\n%v", strings.Join(cmd.Args, " "), string(output))
	cmd = exec.Command(device, "acl", "get", fmt.Sprintf("/%s:8150/device", ip))
	setCredentialsEnv(cmd)
	output, err = cmd.CombinedOutput()
	dieIfErr(err, "Getting device manager acls (%v) failed. Output:\n%v", strings.Join(cmd.Args, " "), string(output))

	fmt.Printf("Done claiming device manager. Device manager ACLs:\n%s", string(output))
}

// installApp installs the binary specified on the command-line and returns the
// Vanadium name for the installation object.
func installApp(deviceName, ip string) string {
	args := []string{fmt.Sprintf("--v23.proxy=/%s:8160", ip), "install-local", deviceName + "/apps", "app"}
	args = append(args, flag.Args()...)
	cmd := exec.Command(device, args...)
	setCredentialsEnv(cmd)
	cmd.Env = append(cmd.Env, fmt.Sprintf("V23_NAMESPACE=/%s:8151", ip))

	// During installation, there are sometimes timeout messages on stderr even when the
	// installation succeeds -- so we need to capture stderr separately from stdout
	buf := new(bytes.Buffer)
	cmd.Stderr = buf
	output, err := cmd.Output()
	dieIfErr(err, "Installing app (%v) failed. Output:\n%v\nStderr:\n%v", strings.Join(cmd.Args, " "), string(output), buf.String())
	installationName := strings.TrimSpace(string(output))

	fmt.Println("Installed", installationName)
	return installationName
}

// startApp creates and launches an instance of the given installation, blessing
// it with extension.  It returns the Vanadium name for the instance object.
func startApp(installationName, extension string) string {
	cmd := exec.Command(device, "instantiate", installationName, extension)
	setCredentialsEnv(cmd)
	output, err := cmd.CombinedOutput()
	dieIfErr(err, "Instantiating app (%v) failed. Output:\n%v", strings.Join(cmd.Args, " "), string(output))
	instanceName := strings.TrimSpace(string(output))
	fmt.Println("Instantiated", instanceName)
	cmd = exec.Command(device, "run", instanceName)
	setCredentialsEnv(cmd)
	output, err = cmd.CombinedOutput()
	dieIfErr(err, "Starting app (%v) failed. Output:\n%v", strings.Join(cmd.Args, " "), string(output))
	return instanceName
}

// check flags and produce the options used to build the backend VM
func handleFlags() (vmOpts interface{}, suppressCleanup bool) {
	var keyDir string
	flag.StringVar(&keyDir, "keydir", "/tmp", "where (on the local filesystem) to store ssh keys")
	var debug bool
	flag.BoolVar(&debug, "debug", false, "print debug messages")
	flag.BoolVar(&suppressCleanup, "suppress-cleanup", false, "suppress cleanup on death to facilitate diagnosis")

	// flags for ssh backend
	var sshTarget, sshOptions string
	flag.StringVar(&sshTarget, "ssh", "", "specify [user@]ip target for ssh")
	flag.StringVar(&sshOptions, "sshoptions", "", "flags to pass to ssh, e.g. \"-i <keyfile>\"")
	// flags for AWS backend
	var aws bool
	var awsRegion, awsImageID string
	flag.BoolVar(&aws, "aws", false, "use AWS backend")
	flag.StringVar(&awsRegion, "aws-region", "", "AWS region -- When specified, you also need to specify aws-image-id")
	flag.StringVar(&awsImageID, "aws-image-id", "", "ID of AWS vm image to use")

	flag.Parse()
	if len(flag.Args()) == 0 {
		die("Usage: %s [--ssh [user@]ip] [--sshoptions \"<options>\"]  <app> <arguments ... >\n", os.Args[0])
	}
	binPath := flag.Args()[0]
	if fi, err := os.Stat(binPath); err != nil {
		die("failed to stat %v: %v", binPath, err)
	} else if fi.IsDir() {
		die("%v is a directory", binPath)
	}

	var dbg backend.DebugPrinter = backend.NoopDebugPrinter{}
	if debug {
		dbg = backend.StderrDebugPrinter{}
	}

	// Pick backend based on flags
	switch {
	default:
		// Vcloud backend
		vcloud = buildV23Binaries(vcloudBin)[0]
		vmOpts = backend.VcloudVMOptions{VcloudBinary: vcloud}

	case sshTarget != "":
		// Ssh backend
		opts := backend.SSHVMOptions{
			Dbg: dbg,
		}
		targetComponents := strings.Split(sshTarget, "@")
		switch len(targetComponents) {
		case 1:
			opts.SSHHostIP = targetComponents[0]
		case 2:
			opts.SSHUser = targetComponents[0]
			opts.SSHHostIP = targetComponents[1]
		default:
			die("Unable to parse sshTarget: %s\n", sshTarget)
		}
		if sshOptions = strings.TrimSpace(sshOptions); sshOptions != "" {
			opts.SSHOptions = strings.Split(sshOptions, " ")
		}
		vmOpts = opts

	case aws:
		vmOpts = backend.AWSVMOptions{
			AWSBinary: "", // Assume "aws" command is on the path
			Region:    awsRegion,
			ImageID:   awsImageID,
			KeyDir:    keyDir,
			Dbg:       dbg,
		}
	}

	return
}

func main() {
	setupWorkDir()
	cleanupOnDeath = func() {
		os.RemoveAll(workDir)
	}
	defer os.RemoveAll(workDir)
	vmOpts, suppressCleanup := handleFlags()
	dmBinaries := buildV23Binaries(append([]string{deviceBin}, dmBins[:]...)...)
	device, dmBinaries = dmBinaries[0], dmBinaries[1:]
	archive := createArchive(append(dmBinaries, getPath(devicexRemote, devicex)))

	vm, vmInstanceName, vmInstanceIP := setupInstance(vmOpts)
	if suppressCleanup {
		cleanupOnDeath = nil
	} else {
		cleanupOnDeath = func() {
			fmt.Fprintf(os.Stderr, "Attempting to stop restarter/deviced ...\n")
			vm.RunCommand("sudo", "killall", "-9", "restarter", "deviced") // errors are ignored
			fmt.Fprintf(os.Stderr, "Cleaning up VM instance ...\n")
			err := vm.Delete()
			fmt.Fprintf(os.Stderr, "Removing local tmp files ...\n")
			os.RemoveAll(workDir)
			dieIfErr(err, "Cleaning up VM instance failed")
			fmt.Fprintf(os.Stderr, "To keep device manager alive for debugging, run again with --suppress-cleanup.\n")
		}
	}
	installArchive(archive, vmInstanceName)
	publicKey, pairingToken := installDevice(vmInstanceName)
	deviceAddr := net.JoinHostPort(vmInstanceIP, "8150")
	deviceName := "/" + deviceAddr
	claimDevice(deviceName, vmInstanceIP, publicKey, pairingToken, vmInstanceName)
	installationName := installApp(deviceName, vmInstanceIP)
	instanceName := startApp(installationName, "app")
	fmt.Println("Launched app.")
	fmt.Println("-------------")
	fmt.Println("See its status:")
	fmt.Printf("\t${JIRI_ROOT}/release/go/bin/device status %s\n", instanceName)
	fmt.Println("See the logs:")
	fmt.Printf("\t${JIRI_ROOT}/release/go/bin/debug glob %s/logs/*\n", instanceName)
	fmt.Println("Dump e.g. the INFO log:")
	fmt.Printf("\t${JIRI_ROOT}/release/go/bin/debug logs read %s/logs/app.INFO\n", instanceName)

	fmt.Println("Clean up the VM instance:")
	fmt.Printf("\t${JIRI_ROOT}/release/go/bin/device kill %s\n", instanceName)
	fmt.Printf("\t%s\n", vm.RunCommandForUser("sudo", "killall", "-9", "restarter", "deviced"))
	fmt.Printf("\t%s\n", vm.DeleteCommandForUser())
}
