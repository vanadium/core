// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"v.io/v23/services/device"
	"v.io/v23/verror"
	"v.io/x/ref/services/device/internal/errors"
)

func getInstallationState(installationDir string) (device.InstallationState, error) {
	for i := 0; i < 2; i++ {
		// TODO(caprita): This is racy w.r.t. instances that are
		// transitioning states.  We currently do a retry because of
		// this, which in practice should be sufficient; a more
		// deterministic solution would involve changing the way we
		// store status.
		for _, s := range device.InstallationStateAll {
			if installationStateIs(installationDir, s) {
				return s, nil
			}
		}
	}
	return device.InstallationStateActive, verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("failed to determine state for installation in dir %v", installationDir))
}

func installationStateIs(installationDir string, state device.InstallationState) bool {
	if _, err := os.Stat(filepath.Join(installationDir, state.String())); err != nil {
		return false
	}
	return true
}

func transitionInstallation(installationDir string, initial, target device.InstallationState) error {
	return transitionState(installationDir, initial, target)
}

func initializeInstallation(installationDir string, initial device.InstallationState) error {
	return initializeState(installationDir, initial)
}

func getInstanceState(instanceDir string) (device.InstanceState, error) {
	for _, s := range device.InstanceStateAll {
		if instanceStateIs(instanceDir, s) {
			return s, nil
		}
	}
	return device.InstanceStateLaunching, verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("failed to determine state for instance in dir %v", instanceDir))
}

func instanceStateIs(instanceDir string, state device.InstanceState) bool {
	if _, err := os.Stat(filepath.Join(instanceDir, state.String())); err != nil {
		return false
	}
	return true
}

func transitionInstance(instanceDir string, initial, target device.InstanceState) error {
	return transitionState(instanceDir, initial, target)
}

func initializeInstance(instanceDir string, initial device.InstanceState) error {
	return initializeState(instanceDir, initial)
}

func transitionState(dir string, initial, target fmt.Stringer) error {
	initialState := filepath.Join(dir, initial.String())
	targetState := filepath.Join(dir, target.String())
	if err := os.Rename(initialState, targetState); err != nil {
		if os.IsNotExist(err) {
			return verror.New(errors.ErrInvalidOperation, nil, err)
		}
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Rename(%v, %v) failed: %v", initialState, targetState, err))
	}
	return nil
}

func initializeState(dir string, initial fmt.Stringer) error {
	initialStatus := filepath.Join(dir, initial.String())
	if err := ioutil.WriteFile(initialStatus, []byte("status"), 0600); err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("WriteFile(%v) failed: %v", initialStatus, err))
	}
	return nil
}
