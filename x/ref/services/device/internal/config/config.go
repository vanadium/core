// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package config handles configuration state passed across instances of the
// device manager.
//
// The State object captures setting that the device manager needs to be aware
// of when it starts.  This is passed to the first invocation of the device
// manager, and then passed down from old device manager to new device manager
// upon update.  The device manager has an implementation-dependent mechanism
// for parsing and passing state, which is encapsulated by the state sub-package
// (currently, the mechanism uses environment variables).  When instantiating a
// new instance of the device manager service, the developer needs to pass in a
// copy of State.  They can obtain this by calling Load, which captures any
// config state passed by a previous version of device manager during update.
// Any new version of the device manager must be able to decode a previous
// version's config state, even if the new version changes the mechanism for
// passing this state (that is, device manager implementations must be
// backward-compatible as far as accepting and passing config state goes).
// TODO(caprita): add config state versioning?
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/services/application"
	"v.io/v23/verror"
	"v.io/x/ref"
)

const pkgPath = "v.io/x/ref/services/device/internal/config"

var (
	errNeedName           = verror.Register(pkgPath+".errNeedName", verror.NoRetry, "{1:}{2:} Name cannot be empty{:_}")
	errNeedRoot           = verror.Register(pkgPath+".errNeedRoot", verror.NoRetry, "{1:}{2:} Root cannot be empty{:_}")
	errNeedCurrentLink    = verror.Register(pkgPath+".errNeedCurrentLink", verror.NoRetry, "{1:}{2:} CurrentLink cannot be empty{:_}")
	errNeedHelper         = verror.Register(pkgPath+".errNeedHelper", verror.NoRetry, "{1:}{2:} Helper must be specified{:_}")
	errCantDecodeEnvelope = verror.Register(pkgPath+".errCantDecodeEnvelope", verror.NoRetry, "{1:}{2:} failed to decode envelope from {3}{:_}")
	errCantEncodeEnvelope = verror.Register(pkgPath+".errCantEncodeEnvelope", verror.NoRetry, "{1:}{2:} failed to encode envelope {3}{:_}")
	errEvalSymlinksFailed = verror.Register(pkgPath+".errEvalSymlinksFailed", verror.NoRetry, "{1:}{2:} EvalSymlinks failed{:_}")
)

// State specifies how the device manager is configured.  This should
// encapsulate what the device manager needs to know and/or be able to mutate
// about its environment.
type State struct {
	// Name is the device manager's object name.  Must be non-empty.
	Name string
	// Envelope is the device manager's application envelope.  If nil, any
	// envelope fetched from the application repository will trigger an
	// update.
	Envelope *application.Envelope
	// Previous holds the local path to the previous version of the device
	// manager.  If empty, revert is disabled.
	Previous string
	// Root is the directory on the local filesystem that contains
	// the applications' workspaces.  Must be non-empty.
	Root string
	// Origin is the application repository object name for the device
	// manager application.  If empty, update is disabled.
	Origin string
	// CurrentLink is the local filesystem soft link that should point to
	// the version of the device manager binary/script through which device
	// manager is started.  Device manager is expected to mutate this during
	// a self-update.  Must be non-empty.
	CurrentLink string
	// Helper is the path to the setuid helper for running applications as
	// specific users.
	Helper string
}

// Validate checks the config state.
func (c *State) Validate() error {
	if c.Name == "" {
		return verror.New(errNeedName, nil)
	}
	if c.Root == "" {
		return verror.New(errNeedRoot, nil)
	}
	if c.CurrentLink == "" {
		return verror.New(errNeedCurrentLink, nil)
	}
	if c.Helper == "" {
		return verror.New(errNeedHelper, nil)
	}
	return nil
}

// Load reconstructs the config state passed to the device manager (presumably
// by the parent device manager during an update).  Currently, this is done via
// environment variables.
func Load() (*State, error) {
	var env *application.Envelope
	if jsonEnvelope := os.Getenv(EnvelopeEnv); jsonEnvelope != "" {
		env = new(application.Envelope)
		if err := json.Unmarshal([]byte(jsonEnvelope), env); err != nil {
			return nil, verror.New(errCantDecodeEnvelope, nil, jsonEnvelope, err)
		}
	}
	return &State{
		Envelope:    env,
		Previous:    os.Getenv(PreviousEnv),
		Root:        os.Getenv(RootEnv),
		Origin:      os.Getenv(OriginEnv),
		CurrentLink: os.Getenv(CurrentLinkEnv),
		Helper:      os.Getenv(HelperEnv),
	}, nil
}

// Save serializes the config state meant to be passed to a child device manager
// during an update, returning a slice of "key=value" strings, which are
// expected to be stuffed into environment variable settings by the caller.
func (c *State) Save(envelope *application.Envelope) ([]string, error) {
	var jsonEnvelope []byte
	if envelope != nil {
		var err error
		if jsonEnvelope, err = json.Marshal(envelope); err != nil {
			return nil, verror.New(errCantEncodeEnvelope, nil, envelope, err)
		}
	}
	var currScript string
	if _, err := os.Lstat(c.CurrentLink); !os.IsNotExist(err) {
		if currScript, err = filepath.EvalSymlinks(c.CurrentLink); err != nil {
			return nil, verror.New(errEvalSymlinksFailed, nil, err)
		}
	}
	settings := map[string]string{
		EnvelopeEnv:    string(jsonEnvelope),
		PreviousEnv:    currScript,
		RootEnv:        c.Root,
		OriginEnv:      c.Origin,
		CurrentLinkEnv: c.CurrentLink,
		HelperEnv:      c.Helper,
	}
	// We need to manually pass the namespace roots to the child, since we
	// currently don't have a way for the child to obtain this information
	// from a config service at start-up.
	roots, _ := ref.EnvNamespaceRoots()
	var ret []string
	for k, v := range roots {
		ret = append(ret, k+"="+v)
	}
	for k, v := range settings {
		ret = append(ret, k+"="+v)
	}
	return ret, nil
}

// QuoteEnv wraps environment variable values in double quotes, making them
// suitable for inclusion in a bash script.
func QuoteEnv(env []string) (ret []string) {
	for _, e := range env {
		if eqIdx := strings.Index(e, "="); eqIdx > 0 {
			ret = append(ret, fmt.Sprintf("%s=%q", e[:eqIdx], e[eqIdx+1:]))
		} else {
			ret = append(ret, e)
		}
	}
	return
}
