// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

// The device invoker is responsible for managing the state of the device
// manager itself.  The implementation expects that the device manager
// installations are all organized in the following directory structure:
//
// <config.Root>/
//   device-manager/
//     info                    - metadata for the device manager (such as object
//                               name and process id)
//     logs/                   - device manager logs
//       STDERR-<timestamp>    - one for each execution of device manager
//       STDOUT-<timestamp>    - one for each execution of device manager
//     <version 1 timestamp>/  - timestamp of when the version was downloaded
//       deviced               - the device manager binary
//       deviced.sh            - a shell script to start the binary
//     <version 2 timestamp>
//     ...
//     device-data/
//       acls/
//         data
//         signature
//     associated.accounts
//       persistent-args       - list of persistent arguments for the device
//                               manager (json encoded)
//
// The device manager is always expected to be started through the symbolic link
// passed in as config.CurrentLink, which is monitored by an init daemon. This
// provides for simple and robust updates.
//
// To update the device manager to a newer version, a new workspace is created
// and the symlink is updated to the new deviced.sh script. Similarly, to revert
// the device manager to a previous version, all that is required is to update
// the symlink to point to the previous deviced.sh script.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/services/binary"
	"v.io/v23/services/device"
	"v.io/v23/verror"
	vexec "v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/mgmt"
	"v.io/x/ref/services/device/internal/config"
	"v.io/x/ref/services/device/internal/errors"
	"v.io/x/ref/services/profile"
)

type updatingState struct {
	// updating is a flag that records whether this instance of device
	// manager is being updated.
	updating bool
	// updatingMutex is a lock for coordinating concurrent access to
	// <updating>.
	updatingMutex sync.Mutex
}

func newUpdatingState() *updatingState {
	return new(updatingState)
}

func (u *updatingState) testAndSetUpdating() bool {
	u.updatingMutex.Lock()
	defer u.updatingMutex.Unlock()
	if u.updating {
		return true
	}
	u.updating = true
	return false
}

func (u *updatingState) unsetUpdating() {
	u.updatingMutex.Lock()
	u.updating = false
	u.updatingMutex.Unlock()
}

// deviceService implements the Device manager's Device interface.
type deviceService struct {
	updating       *updatingState
	restartHandler func()
	callback       *callbackState
	config         *config.State
	disp           *dispatcher
	uat            BlessingSystemAssociationStore
	principalMgr   principalManager
	tidying        chan<- tidyRequests
}

// ManagerInfo holds state about a running device manager or a running restarter
type ManagerInfo struct {
	Pid int
}

func SaveManagerInfo(dir string, info *ManagerInfo) error {
	jsonInfo, err := json.Marshal(info)
	if err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Marshal(%v) failed: %v", info, err))
	}
	if err := os.MkdirAll(dir, os.FileMode(0700)); err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("MkdirAll(%v) failed: %v", dir, err))
	}
	infoPath := filepath.Join(dir, "info")
	if err := ioutil.WriteFile(infoPath, jsonInfo, 0600); err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("WriteFile(%v) failed: %v", infoPath, err))
	}
	return nil
}

func LoadManagerInfo(dir string) (*ManagerInfo, error) {
	infoPath := filepath.Join(dir, "info")
	info := new(ManagerInfo)
	if infoBytes, err := ioutil.ReadFile(infoPath); err != nil {
		return nil, verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("ReadFile(%v) failed: %v", infoPath, err))
	} else if err := json.Unmarshal(infoBytes, info); err != nil {
		return nil, verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Unmarshal(%v) failed: %v", infoBytes, err))
	}
	return info, nil
}

func SavePersistentArgs(root string, args []string) error {
	dir := filepath.Join(root, "device-manager", "device-data")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("MkdirAll(%q) failed: %v", dir, err)
	}
	data, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("Marshal(%v) failed: %v", args, err)
	}
	fileName := filepath.Join(dir, "persistent-args")
	return ioutil.WriteFile(fileName, data, 0600)
}

func loadPersistentArgs(root string) ([]string, error) {
	fileName := filepath.Join(root, "device-manager", "device-data", "persistent-args")
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	args := []string{}
	if err := json.Unmarshal(bytes, &args); err != nil {
		return nil, fmt.Errorf("json.Unmarshal(%v) failed: %v", bytes, err)
	}
	return args, nil
}

func (*deviceService) Describe(*context.T, rpc.ServerCall) (device.Description, error) {
	return Describe()
}

func (*deviceService) IsRunnable(_ *context.T, _ rpc.ServerCall, description binary.Description) (bool, error) {
	deviceProfile, err := ComputeDeviceProfile()
	if err != nil {
		return false, err
	}
	binaryProfiles := make([]*profile.Specification, 0)
	for name, _ := range description.Profiles {
		profile, err := getProfile(name)
		if err != nil {
			return false, err
		}
		binaryProfiles = append(binaryProfiles, profile)
	}
	result := matchProfiles(deviceProfile, binaryProfiles)
	return len(result.Profiles) > 0, nil
}

func (*deviceService) Reset(_ *context.T, _ rpc.ServerCall, deadline time.Duration) error {
	// TODO(jsimsa): Implement.
	return nil
}

// getCurrentFileInfo returns the os.FileInfo for both the symbolic link
// CurrentLink, and the device script in the workspace that this link points to.
func (s *deviceService) getCurrentFileInfo() (os.FileInfo, string, error) {
	path := s.config.CurrentLink
	link, err := os.Lstat(path)
	if err != nil {
		return nil, "", verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Lstat(%v) failed: %v", path, err))
	}
	scriptPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, "", verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("EvalSymlinks(%v) failed: %v", path, err))
	}
	return link, scriptPath, nil
}

func (s *deviceService) revertDeviceManager(ctx *context.T) error {
	if err := UpdateLink(s.config.Previous, s.config.CurrentLink); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("UpdateLink failed: %v", err))
	}
	if s.restartHandler != nil {
		s.restartHandler()
	}
	v23.GetAppCycle(ctx).Stop(ctx)
	return nil
}

func (s *deviceService) newLogfile(prefix string) (*os.File, error) {
	d := filepath.Join(s.config.Root, "device_test_logs")
	if _, err := os.Stat(d); err != nil {
		if err := os.MkdirAll(d, 0700); err != nil {
			return nil, err
		}
	}
	f, err := ioutil.TempFile(d, "__device_impl_test__"+prefix)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// TODO(cnicolaou): would this be better implemented using the v23test/gosh
// framework now that it exists?
func (s *deviceService) testDeviceManager(ctx *context.T, workspace string, envelope *application.Envelope) error {
	path := filepath.Join(workspace, "deviced.sh")
	cmd := exec.Command(path)
	cmd.Env = []string{"DEVICE_MANAGER_DONT_REDIRECT_STDOUT_STDERR=1"}

	for k, v := range map[string]*io.Writer{
		"stdout": &cmd.Stdout,
		"stderr": &cmd.Stderr,
	} {
		// Using a log file makes it less likely that stdout and stderr
		// output will be lost if the child crashes.
		file, err := s.newLogfile(fmt.Sprintf("deviced-test-%s", k))
		if err != nil {
			return err
		}
		fName := file.Name()
		defer os.Remove(fName)
		*v = file

		defer func(k string) {
			if f, err := os.Open(fName); err == nil {
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					ctx.Infof("[testDeviceManager %s] %s", k, scanner.Text())
				}
			}
		}(k)
	}

	// Setup up the child process callback.
	callbackState := s.callback
	listener := callbackState.listenFor(ctx, mgmt.ChildNameConfigKey)
	defer listener.cleanup()
	cfg := vexec.NewConfig()

	cfg.Set(mgmt.ParentNameConfigKey, listener.name())
	cfg.Set(mgmt.ProtocolConfigKey, "tcp")
	cfg.Set(mgmt.AddressConfigKey, "127.0.0.1:0")

	principalMgr := s.principalMgr
	if err := principalMgr.Create(workspace); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Create(%v) failed: %v", workspace, err))
	}
	defer principalMgr.Delete(workspace)
	if err := principalMgr.Serve(workspace, cfg); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Serve(%v) failed: %v", workspace, err))
	}
	defer principalMgr.StopServing(workspace)
	p, err := principalMgr.Load(workspace)
	if err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Load(%v) failed: %v", workspace, err))
	}
	defer p.Close()
	dmPrincipal := v23.GetPrincipal(ctx)
	dmBlessings, _ := dmPrincipal.BlessingStore().Default()
	testDmBlessings, err := dmPrincipal.Bless(p.PublicKey(), dmBlessings, "testdm", security.UnconstrainedUse())
	if err := p.BlessingStore().SetDefault(testDmBlessings); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("BlessingStore.SetDefault() failed: %v", err))
	}
	if _, err := p.BlessingStore().Set(testDmBlessings, security.AllPrincipals); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("BlessingStore.Set() failed: %v", err))
	}
	if err := security.AddToRoots(p, testDmBlessings); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("AddToRoots() failed: %v", err))
	}

	env, err := vexec.WriteConfigToEnv(cfg, cmd.Env)
	if err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("encoding config failed %v", err))
	}
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		ctx.Errorf("Start() failed: %v", err)
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Start() failed: %v", err))
	}

	// Watch for the exit of the child. Failures could cause it to happen at any time
	waitchan := make(chan error, 1)
	go func() {
		if err := cmd.Wait(); err != nil {
			waitchan <- verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("new device manager failed to exit cleanly: %v", err))
		}
		close(waitchan)
		listener.stop()
	}()

	childName, err := listener.waitForValue(childReadyTimeout)
	if err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("waitForValue(%v) failed: %v", childReadyTimeout, err))
	}
	// Check that invoking Delete() succeeds.
	childName = naming.Join(childName, "device")
	dmClient := device.DeviceClient(childName)
	if err := dmClient.Delete(ctx); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Delete() failed: %v", err))
	}
	select {
	case err := <-waitchan:
		return err // err is nil if cmd.Wait succceeded
	case <-time.After(childWaitTimeout):
		return verror.New(errors.ErrOperationFailed, ctx, "new device manager failed to run in allotted time")
	}
	return nil
}

// TODO(caprita): Move this to util.go since device_installer is also using it now.

func GenerateScript(workspace string, configSettings []string, envelope *application.Envelope, logs string) error {
	// TODO(caprita): Remove this snippet of code, it doesn't seem to serve
	// any purpose.
	path, err := filepath.EvalSymlinks(os.Args[0])
	if err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("EvalSymlinks(%v) failed: %v", os.Args[0], err))
	}

	if err := os.MkdirAll(logs, 0711); err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("MkdirAll(%v) failed: %v", logs, err))
	}
	stderrLog, stdoutLog := filepath.Join(logs, "STDERR"), filepath.Join(logs, "STDOUT")

	output := "#!" + ShellPath + "\n"
	output += "if [ -z \"$DEVICE_MANAGER_DONT_REDIRECT_STDOUT_STDERR\" ]; then\n"
	output += fmt.Sprintf("  TIMESTAMP=$(%s)\n", DateCommand)
	output += fmt.Sprintf("  exec > %s-$TIMESTAMP 2> %s-$TIMESTAMP\n", stdoutLog, stderrLog)
	output += "  LOG_TO_STDERR=false\n"
	output += "else\n"
	output += "  LOG_TO_STDERR=true\n"
	output += "fi\n"
	output += strings.Join(config.QuoteEnv(append(envelope.Env, configSettings...)), " ") + " "
	// Escape the path to the binary; %q uses Go-syntax escaping, but it's
	// close enough to Bash that we're using it as an approximation.
	//
	// TODO(caprita/rthellend): expose and use shellEscape (from
	// v.io/x/ref/services/debug/debug/impl.go) instead.
	output += fmt.Sprintf("exec %q", filepath.Join(workspace, "deviced")) + " "
	output += fmt.Sprintf("--log_dir=%q ", logs)
	output += "--logtostderr=${LOG_TO_STDERR} "
	output += strings.Join(envelope.Args, " ")

	path = filepath.Join(workspace, "deviced.sh")
	if err := ioutil.WriteFile(path, []byte(output), 0700); err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("WriteFile(%v) failed: %v", path, err))
	}
	return nil
}

func (s *deviceService) updateDeviceManager(ctx *context.T) error {
	if len(s.config.Origin) == 0 {
		return verror.New(errors.ErrUpdateNoOp, ctx)
	}
	envelope, err := fetchEnvelope(ctx, s.config.Origin)
	if err != nil {
		return err
	}
	if envelope.Title != application.DeviceManagerTitle {
		return verror.New(errors.ErrAppTitleMismatch, ctx, fmt.Sprintf("app title mismatch. Got %q, expected %q.", envelope.Title, application.DeviceManagerTitle))
	}
	// Read and merge persistent args, if present.
	if args, err := loadPersistentArgs(s.config.Root); err == nil {
		envelope.Args = append(envelope.Args, args...)
	}
	if s.config.Envelope != nil && reflect.DeepEqual(envelope, s.config.Envelope) {
		return verror.New(errors.ErrUpdateNoOp, ctx)
	}
	// Create new workspace.
	workspace := filepath.Join(s.config.Root, "device-manager", generateVersionDirName())
	perm := os.FileMode(0700)
	if err := os.MkdirAll(workspace, perm); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("MkdirAll(%v, %v) failed: %v", workspace, perm, err))
	}

	deferrer := func() {
		CleanupDir(ctx, workspace, "")
	}
	defer func() {
		if deferrer != nil {
			deferrer()
		}
	}()

	// Populate the new workspace with a device manager binary.
	// TODO(caprita): match identical binaries on binary signature
	// rather than binary object name.
	sameBinary := s.config.Envelope != nil && envelope.Binary.File == s.config.Envelope.Binary.File
	if sameBinary {
		if err := LinkSelf(workspace, "deviced"); err != nil {
			return err
		}
	} else {
		if err := downloadBinary(ctx, envelope.Publisher, &envelope.Binary, workspace, "deviced"); err != nil {
			return err
		}
	}

	// Populate the new workspace with a device manager script.
	configSettings, err := s.config.Save(envelope)
	if err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, err)
	}

	logs := filepath.Join(s.config.Root, "device-manager", "logs")
	if err := GenerateScript(workspace, configSettings, envelope, logs); err != nil {
		return err
	}

	if err := s.testDeviceManager(ctx, workspace, envelope); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("testDeviceManager failed: %v", err))
	}

	if err := UpdateLink(filepath.Join(workspace, "deviced.sh"), s.config.CurrentLink); err != nil {
		return err
	}

	if s.restartHandler != nil {
		s.restartHandler()
	}
	v23.GetAppCycle(ctx).Stop(ctx)
	deferrer = nil
	return nil
}

func (*deviceService) Install(ctx *context.T, _ rpc.ServerCall, _ string, _ device.Config, _ application.Packages) (string, error) {
	return "", verror.New(errors.ErrInvalidSuffix, ctx)
}

func (*deviceService) Run(ctx *context.T, _ rpc.ServerCall) error {
	return verror.New(errors.ErrInvalidSuffix, ctx)
}

func (s *deviceService) Revert(ctx *context.T, _ rpc.ServerCall) error {
	if s.config.Previous == "" {
		return verror.New(errors.ErrUpdateNoOp, ctx, fmt.Sprintf("Revert failed: no previous version"))
	}
	updatingState := s.updating
	if updatingState.testAndSetUpdating() {
		return verror.New(errors.ErrOperationInProgress, ctx, fmt.Sprintf("Revert failed: already in progress"))
	}
	err := s.revertDeviceManager(ctx)
	if err != nil {
		updatingState.unsetUpdating()
	}
	return err
}

func (*deviceService) Instantiate(ctx *context.T, _ device.ApplicationInstantiateServerCall) (string, error) {
	return "", verror.New(errors.ErrInvalidSuffix, ctx)
}

func (*deviceService) Delete(ctx *context.T, _ rpc.ServerCall) error {
	v23.GetAppCycle(ctx).Stop(ctx)
	return nil
}

func (s *deviceService) Kill(ctx *context.T, _ rpc.ServerCall, _ time.Duration) error {
	if s.restartHandler != nil {
		s.restartHandler()
	}
	v23.GetAppCycle(ctx).Stop(ctx)
	return nil
}

func (*deviceService) Uninstall(ctx *context.T, _ rpc.ServerCall) error {
	return verror.New(errors.ErrInvalidSuffix, ctx)
}

func (s *deviceService) Update(ctx *context.T, _ rpc.ServerCall) error {
	ctx, cancel := context.WithTimeout(ctx, rpcContextLongTimeout)
	defer cancel()

	updatingState := s.updating
	if updatingState.testAndSetUpdating() {
		return verror.New(errors.ErrOperationInProgress, ctx)
	}

	err := s.updateDeviceManager(ctx)
	if err != nil {
		updatingState.unsetUpdating()
	}
	return err
}

func (*deviceService) UpdateTo(*context.T, rpc.ServerCall, string) error {
	// TODO(jsimsa): Implement.
	return nil
}

func (s *deviceService) SetPermissions(_ *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error {
	d := PermsDir(s.disp.config)
	return s.disp.permsStore.Set(d, perms, version)
}

func (s *deviceService) GetPermissions(*context.T, rpc.ServerCall) (perms access.Permissions, version string, err error) {
	d := PermsDir(s.disp.config)
	return s.disp.permsStore.Get(d)
}

// TODO(rjkroege): Make it possible for users on the same system to also
// associate their accounts with their identities.
func (s *deviceService) AssociateAccount(_ *context.T, _ rpc.ServerCall, identityNames []string, accountName string) error {
	if accountName == "" {
		return s.uat.DisassociateSystemAccountForBlessings(identityNames)
	}
	// TODO(rjkroege): Optionally verify here that the required uname is a valid.
	return s.uat.AssociateSystemAccountForBlessings(identityNames, accountName)
}

func (s *deviceService) ListAssociations(ctx *context.T, call rpc.ServerCall) (associations []device.Association, err error) {
	// Temporary code. Dump this.
	if ctx.V(2) {
		b, r := security.RemoteBlessingNames(ctx, call.Security())
		ctx.Infof("ListAssociations given blessings: %v\n", b)
		if len(r) > 0 {
			ctx.Infof("ListAssociations rejected blessings: %v\n", r)
		}
	}
	return s.uat.AllBlessingSystemAssociations()
}

func (*deviceService) Debug(*context.T, rpc.ServerCall) (string, error) {
	return "Not implemented", nil
}

func (s *deviceService) Status(*context.T, rpc.ServerCall) (device.Status, error) {
	state := device.InstanceStateRunning
	if s.updating.updating {
		state = device.InstanceStateUpdating
	}
	// Extract the version from the current link path.
	//
	// TODO(caprita): make the version available in the device's directory.
	scriptPath, err := filepath.EvalSymlinks(s.config.CurrentLink)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(scriptPath)
	versionDir := filepath.Base(dir)
	if versionDir == "." {
		versionDir = "base"
	}
	return device.StatusDevice{Value: device.DeviceStatus{
		State:   state,
		Version: versionDir,
	}}, nil
}

func (s *deviceService) TidyNow(ctx *context.T, _ rpc.ServerCall) error {
	ec := make(chan error)
	s.tidying <- tidyRequests{ctx: ctx, bc: ec}
	return <-ec
}
