// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref"
	"v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/mgmt"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/services/agent"
	"v.io/x/ref/services/agent/agentlib"
)

var (
	errCredentialsInit = verror.Register(pkgPath+".errCredentialsInit", verror.NoRetry, "failed to initialize credentials, perhaps you need to create them with 'principal create {3}' (error: {4})")
)

func (r *Runtime) initPrincipal(ctx *context.T, credentials string) (security.Principal, func(), error) {
	if principal, _ := ctx.Value(principalKey).(security.Principal); principal != nil {
		return principal, func() {}, nil
	}
	if len(credentials) > 0 {
		// Explicitly specified credentials, use (or set up) an agent
		// that serves the principal; or otherwise load the principal
		// exclusively.
		principal, err := agentlib.LoadPrincipal(credentials)
		if err != nil {
			return nil, nil, verror.New(errCredentialsInit, ctx, credentials, err)
		}
		return principal, func() { principal.Close() }, nil
	}
	// Use credentials stored in the agent.
	if principal, err := ipcAgent(); err != nil {
		return nil, nil, err
	} else if principal != nil {
		return principal, func() { principal.Close() }, nil
	}
	// No agent, no explicit credentials specified: create a new principal
	// and blessing in memory.
	principal, err := vsecurity.NewPrincipal()
	if err != nil {
		return principal, nil, err
	}
	return principal, func() {}, vsecurity.InitDefaultBlessings(principal, defaultBlessingName())
}

func ipcAgent() (agent.Principal, error) {
	var config exec.Config
	config, err := exec.ReadConfigFromOSEnv()
	if err != nil {
		return nil, err
	}
	var path string
	if config != nil {
		// We were started by a parent (presumably, device manager).
		path, _ = config.Get(mgmt.SecurityAgentPathConfigKey)
	} else {
		path = os.Getenv(ref.EnvAgentPath)
	}
	if path == "" {
		return nil, nil
	}
	return agentlib.NewAgentPrincipalX(path)
}

func defaultBlessingName() string {
	options := []string{
		"apple", "banana", "cherry", "dragonfruit", "elderberry", "fig", "grape", "honeydew",
	}
	name := fmt.Sprintf("anonymous-%s-%d",
		options[rand.New(rand.NewSource(time.Now().Unix())).Intn(len(options))],
		os.Getpid())
	host, _ := os.Hostname()
	// (none) is a common default hostname and contains parentheses,
	// which are invalid blessings characters.
	if host == "(none)" || len(host) == 0 {
		return name
	}
	return name + "@" + host
}
