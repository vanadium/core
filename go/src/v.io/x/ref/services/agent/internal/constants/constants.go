// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package constants holds constants shared by client and server.
package constants

import "path/filepath"

const (
	agentDir                = "agent"
	socketFile              = "sock"
	ServingMsg              = "serving"
	DaemonFlag              = "daemon"
	TimeoutFlag             = "timeout"
	VersionFlag             = "with-version"
	CredentialsFlag         = "credentials"
	EnvAgentParentPipeFD    = "V23_AGENT_PARENT_PIPE_FD"
	EnvAgentNoPrintCredsEnv = "V23_AGENT_NO_PRINT_CREDS_ENV"
)

// SocketPath returns the location where the agent generates the socket file.
func SocketPath(credsDir string) string {
	return filepath.Join(AgentDir(credsDir), socketFile)
}

// AgentDir returns the directory where the agent keeps its state.
func AgentDir(credsDir string) string {
	return filepath.Join(credsDir, agentDir)
}
