// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"fmt"
	"os"
	"path/filepath"

	"v.io/v23/security"
	"v.io/x/ref/lib/exec"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/services/agent"
)

// principalManager manages security principals.
type principalManager interface {
	// Create creates a new principal for the instance directory.
	Create(instanceDir string) error
	// Delete deletes the principal for the instance directory.
	Delete(instanceDir string) error
	// Load returns the principal for the instance directory.  Assumes the
	// principal is Create-d and Serve-ing.
	Load(instanceDir string) (agent.Principal, error)
	// Serve makes the principal available for Load-in or for use by a child
	// app.  The config object, if not nil, is mutated to make the principal
	// available for a child process.
	Serve(instanceDir string, config exec.Config) error
	// StopServing undoes Serve.
	StopServing(instanceDir string) error
	// Debug returns a debugging information about the principal.
	Debug(instanceDir string) string
}

func newPrincipalManager() principalManager {
	return &diskCredsPM{}
}

// diskCredsPM is a principalManager implementation that uses on-disk
// credentials.  The credentials are not shared via the security agent, and are
// not locked against concurrent access.
type diskCredsPM struct{}

type noOpClosePrincipal struct {
	security.Principal
}

func (p noOpClosePrincipal) Close() error {
	return nil
}

func (pm *diskCredsPM) Create(instanceDir string) error {
	credentialsDir := filepath.Join(instanceDir, "credentials")
	// TODO(caprita): The app's system user id needs access to this dir.
	// Use the suidhelper to chown it.
	_, err := vsecurity.CreatePersistentPrincipal(credentialsDir, nil)
	return err
}

func (pm *diskCredsPM) Delete(instanceDir string) error {
	credentialsDir := filepath.Join(instanceDir, "credentials")
	return os.RemoveAll(credentialsDir)
}

func (pm *diskCredsPM) Load(instanceDir string) (agent.Principal, error) {
	credentialsDir := filepath.Join(instanceDir, "credentials")
	// TODO(caprita): The app's system user id needs access to this dir.
	// Use the suidhelper to chown it.
	p, err := vsecurity.LoadPersistentPrincipal(credentialsDir, nil)
	if err != nil {
		return nil, err
	}
	return noOpClosePrincipal{p}, nil
}

func (pm *diskCredsPM) Serve(instanceDir string, cfg exec.Config) error {
	if cfg != nil {
		cfg.Set("v23.credentials", filepath.Join(instanceDir, "credentials"))
	}
	return nil
}

func (pm *diskCredsPM) StopServing(instanceDir string) error {
	return nil
}

func (pm *diskCredsPM) Debug(instanceDir string) string {
	credentialsDir := filepath.Join(instanceDir, "credentials")
	return fmt.Sprintf("Credentials dir-based (%v)", credentialsDir)
}
