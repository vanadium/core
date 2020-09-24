// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"

	"v.io/x/ref/internal/logger"
	"v.io/x/ref/services/discharger"
	"v.io/x/ref/services/role"
)

func useOrCreateErrInternal(ctx *context.T, err error) error {
	if verror.IsAny(err) {
		return err
	}
	return verror.ErrInternal.Errorf(ctx, "internal error: %v", err)
}

const requiredSuffix = security.ChainSeparator + role.RoleSuffix

// NewDispatcher returns a dispatcher object for a role service and its
// associated discharger service.
// The configRoot is the top level directory where the role configuration files
// are stored.
// The dischargerLocation is the object name or address of the discharger
// service for the third-party caveats attached to the role blessings returned
// by the role service.
func NewDispatcher(configRoot, dischargerLocation string) rpc.Dispatcher {
	return &dispatcher{&serverConfig{configRoot, dischargerLocation}}
}

type serverConfig struct {
	root               string
	dischargerLocation string
}

type dispatcher struct {
	config *serverConfig
}

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	if len(suffix) == 0 {
		return discharger.DischargerServer(&dischargerImpl{d.config}), security.AllowEveryone(), nil
	}
	fileName := filepath.Join(d.config.root, filepath.FromSlash(suffix+".conf"))
	if !strings.HasPrefix(fileName, d.config.root) {
		// Guard against ".." in the suffix that could be used to read
		// files outside of the config root.
		return nil, nil, verror.ErrNoExistOrNoAccess.Errorf(nil, "does not exist or access denied")
	}
	roleConfig, err := loadExpandedConfig(fileName, nil)
	if err != nil && !os.IsNotExist(err) {
		// The config file exists, but we failed to read it for some
		// reason. This is likely a server configuration error.
		logger.Global().Errorf("loadConfig(%q, %q): %v", d.config.root, suffix, err)
		return nil, nil, useOrCreateErrInternal(nil, err)
	}
	obj := &roleService{serverConfig: d.config, role: suffix, roleConfig: roleConfig}
	return role.RoleServer(obj), &authorizer{roleConfig}, nil
}

type authorizer struct {
	config *Config
}

func (a *authorizer) Authorize(ctx *context.T, call security.Call) error {
	if call.Method() == "__Glob" {
		// The Glob implementation only shows objects that the caller
		// has access to. So this blanket approval is OK.
		return nil
	}
	if a.config == nil {
		return verror.ErrNoExistOrNoAccess.Errorf(ctx, "does not exist or access denied")
	}
	remoteBlessingNames, _ := security.RemoteBlessingNames(ctx, call)

	if hasAccess(a.config, remoteBlessingNames) {
		return nil
	}
	return verror.ErrNoExistOrNoAccess.Errorf(ctx, "does not exist or access denied")
}

func hasAccess(c *Config, blessingNames []string) bool {
	for _, pattern := range c.Members {
		if pattern.MatchedBy(blessingNames...) {
			return true
		}
	}
	return false
}

func loadExpandedConfig(fileName string, seenFiles map[string]struct{}) (*Config, error) {
	if seenFiles == nil {
		seenFiles = make(map[string]struct{})
	}
	if _, seen := seenFiles[fileName]; seen {
		return nil, nil
	}
	seenFiles[fileName] = struct{}{}
	c, err := loadConfig(fileName)
	if err != nil {
		return nil, err
	}
	parentDir := filepath.Dir(fileName)
	for _, imp := range c.ImportMembers {
		f := filepath.Join(parentDir, filepath.FromSlash(imp+".conf"))
		ic, err := loadExpandedConfig(f, seenFiles)
		if err != nil {
			continue
		}
		if ic == nil {
			continue
		}
		c.Members = append(c.Members, ic.Members...)
	}
	c.ImportMembers = nil
	dedupMembers(c)
	return c, nil
}

func loadConfig(fileName string) (*Config, error) {
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(contents, &c); err != nil {
		return nil, err
	}
	for i, pattern := range c.Members {
		if p := string(pattern); !strings.HasSuffix(p, requiredSuffix) {
			c.Members[i] = security.BlessingPattern(p + requiredSuffix)
		}
	}
	return &c, nil
}

func dedupMembers(c *Config) {
	members := make(map[security.BlessingPattern]struct{})
	for _, m := range c.Members {
		members[m] = struct{}{}
	}
	c.Members = []security.BlessingPattern{}
	for m := range members {
		c.Members = append(c.Members, m)
	}
}
