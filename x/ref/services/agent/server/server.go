// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(caprita): Move to internal.

// Package server contains utilities for serving a principal using a
// socket-based IPC system.
package server

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/passphrase"
	"v.io/x/ref/services/agent/internal/ipc"
	"v.io/x/ref/services/agent/internal/server"
)

const (
	pkgPath         = "v.io/x/ref/services/agent/server"
	agentSocketName = "agent.sock"
)

var (
	errCantReadPassphrase = verror.Register(pkgPath+".errCantReadPassphrase", verror.NoRetry, "{1:}{2:} failed to read passphrase{:_}")
	errNeedPassphrase     = verror.Register(pkgPath+".errNeedPassphrase", verror.NoRetry, "{1:}{2:} Passphrase required for decrypting principal{:_}")
)

// LoadPrincipal returns the principal persisted in the given credentials
// directory.  If the private key is encrypted, it prompts for a decryption
// passphrase.  If the principal doesn't exist and create is true, it creates
// the principal.
func LoadPrincipal(credentials string, create bool) (security.Principal, error) {
	switch p, err := vsecurity.LoadPersistentPrincipal(credentials, nil); {
	case err == nil:
		return p, nil
	case os.IsNotExist(err):
		if !create {
			return nil, err
		}
		return handleDoesNotExist(credentials)
	case verror.ErrorID(err) == vsecurity.ErrPassphraseRequired.ID:
		return handlePassphrase(credentials)
	default:
		return nil, err
	}
}

// IPCState represents the IPC system serving the principal.
type IPCState interface {
	// Close shuts the IPC system down.
	Close()
	// IdleStartTime returns the time when the IPC system became idle (no
	// connections).  Returns the zero time instant if connections exits.
	IdleStartTime() time.Time
	// NumConnections returns the number of current connections.
	NumConnections() int
}

type ipcState struct {
	*ipc.IPC
}

// NumConnections implements IPCState.NumConnections.
func (s ipcState) NumConnections() int {
	return len(s.IPC.Connections())
}

// Serve serves the given principal using the given socket file, and returns an
// IPCState for the service.
func Serve(p security.Principal, socketPath string) (IPCState, error) {
	var err error
	if socketPath, err = filepath.Abs(socketPath); err != nil {
		return nil, fmt.Errorf("Abs failed: %v", err)
	}
	socketPath = filepath.Clean(socketPath)

	// Start running our server.
	i := ipc.NewIPC()
	if err := server.ServeAgent(i, p); err != nil {
		i.Close()
		return nil, fmt.Errorf("ServeAgent failed: %v", err)
	}
	if err = os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		i.Close()
		return nil, err
	}
	if err := i.Listen(socketPath); err != nil {
		i.Close()
		return nil, fmt.Errorf("Listen failed: %v", err)
	}
	return ipcState{i}, nil
}

func handleDoesNotExist(dir string) (security.Principal, error) {
	fmt.Println("Private key file does not exist. Creating new private key...")
	pass, err := passphrase.Get("Enter passphrase (entering nothing will store unencrypted): ")
	if err != nil {
		return nil, verror.New(errCantReadPassphrase, nil, err)
	}
	defer func() {
		// Zero passhphrase out so it doesn't stay in memory.
		for i := range pass {
			pass[i] = 0
		}
	}()
	p, err := vsecurity.CreatePersistentPrincipal(dir, pass)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func handlePassphrase(dir string) (security.Principal, error) {
	pass, err := passphrase.Get(fmt.Sprintf("Passphrase required to decrypt encrypted private key file for credentials in %v.\nEnter passphrase: ", dir))
	if err != nil {
		return nil, verror.New(errCantReadPassphrase, nil, err)
	}
	p, err := vsecurity.LoadPersistentPrincipal(dir, pass)
	// Zero passhphrase out so it doesn't stay in memory.
	for i := range pass {
		pass[i] = 0
	}
	return p, err
}
