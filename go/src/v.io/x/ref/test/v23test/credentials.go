// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v23test

import (
	"io/ioutil"
	"path/filepath"

	"v.io/v23/security"
	libsec "v.io/x/ref/lib/security"
	"v.io/x/ref/services/agent/agentlib"
	"v.io/x/ref/services/agent/server"
)

// Credentials represents a principal with a set of blessings. It is designed to
// be implementable using either the filesystem (with a credentials dir) or the
// Vanadium security agent.
type Credentials struct {
	Handle    string
	Principal security.Principal // equal to pm.Principal(Handle)
}

// newCredentials creates a new Credentials.
func newCredentials(pm principalManager) (*Credentials, error) {
	h, err := pm.New()
	if err != nil {
		return nil, err
	}
	p, err := pm.Principal(h)
	if err != nil {
		return nil, err
	}
	return &Credentials{Handle: h, Principal: p}, nil
}

// newRootCredentials creates a new Credentials with a self-signed "root"
// blessing.
func newRootCredentials(pm principalManager) (*Credentials, error) {
	res, err := newCredentials(pm)
	if err != nil {
		return nil, err
	}
	if err := libsec.InitDefaultBlessings(res.Principal, "root"); err != nil {
		return nil, err
	}
	return res, nil
}

func addDefaultBlessings(self, other security.Principal, extensions ...string) error {
	for _, extension := range extensions {
		bself, _ := self.BlessingStore().Default()
		blessings, err := self.Bless(other.PublicKey(), bself, extension, security.UnconstrainedUse())
		if err != nil {
			return err
		}
		bother, _ := other.BlessingStore().Default()
		union, err := security.UnionOfBlessings(bother, blessings)
		if err != nil {
			return err
		}
		if err := libsec.SetDefaultBlessings(other, union); err != nil {
			return err
		}
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// principalManager interface and implementations

// principalManager manages principals.
type principalManager interface {
	// New creates a principal and returns a handle to it.
	New() (string, error)

	// Principal returns the principal for the given handle.
	Principal(handle string) (security.Principal, error)
}

////////////////////////////////////////
// filesystemPrincipalManager

type filesystemPrincipalManager struct {
	rootDir string
}

func newFilesystemPrincipalManager(rootDir string) principalManager {
	return &filesystemPrincipalManager{rootDir: rootDir}
}

func (pm *filesystemPrincipalManager) New() (string, error) {
	dir, err := ioutil.TempDir(pm.rootDir, "")
	if err != nil {
		return "", err
	}
	if _, err := libsec.CreatePersistentPrincipal(dir, nil); err != nil {
		return "", err
	}
	return dir, nil
}

func (m *filesystemPrincipalManager) Principal(handle string) (security.Principal, error) {
	return libsec.LoadPersistentPrincipal(handle, nil)
}

////////////////////////////////////////
// agentPrincipalManager

// agentPrincipalManager creates filesystem-based credentials, starts an agent
// per credentials directory, and passes back the agent socket file as the
// 'handle'.
//
// TODO(caprita): We should pass back the credentials directory itself as the
// handle, and not the socket: clients can use agentlib.LoadPrincipal to
// transparently find the socket and load the credentials.  However, some tests
// currently expect v23test.Shell to set V23_AGENT_PATH instead of
// V23_CREDENTIALS on the commands it starts, and we need to fix those tests
// first.

type agentPrincipalManager struct {
	rootDir string
}

func newAgentPrincipalManager(rootDir string) (principalManager, error) {
	return &agentPrincipalManager{rootDir: rootDir}, nil
}

func (pm *agentPrincipalManager) New() (string, error) {
	credsDir, err := ioutil.TempDir(pm.rootDir, "")
	if err != nil {
		return "", err
	}
	p, err := libsec.CreatePersistentPrincipal(credsDir, nil)
	if err != nil {
		return "", err
	}
	sockDir, err := ioutil.TempDir(pm.rootDir, "")
	if err != nil {
		return "", err
	}
	sockPath := filepath.Join(sockDir, "s")
	if _, err = server.Serve(p, sockPath); err != nil {
		return "", err
	}
	return sockPath, nil
}

func (m *agentPrincipalManager) Principal(handle string) (security.Principal, error) {
	return agentlib.NewAgentPrincipalX(handle)
}
