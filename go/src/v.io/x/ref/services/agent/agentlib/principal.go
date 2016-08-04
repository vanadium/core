// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package agentlib

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/lib/metadata"
	"v.io/x/lib/vlog"
	"v.io/x/ref"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/services/agent"
	"v.io/x/ref/services/agent/internal/constants"
	"v.io/x/ref/services/agent/internal/launcher"
	"v.io/x/ref/services/agent/internal/lock"
	"v.io/x/ref/services/agent/internal/version"
)

var (
	errNotADirectory = verror.Register(pkgPath+".errNotADirectory", verror.NoRetry, "{1:}{2:} {3} is not a directory{:_}")
	errFindAgent     = verror.Register(pkgPath+".errFindAgent", verror.NoRetry, "{1:}{2:} couldn't load credentials ({4}) or start an agent ({3})")
	errLaunchAgent   = verror.Register(pkgPath+".errLaunchAgent", verror.NoRetry, "{1:}{2:} couldn't launch agent ({3}) or load principal locally ({4})")
	errLoadLocally   = verror.Register(pkgPath+".errLoadLocally", verror.NoRetry, "{1:}{2:} couldn't load principal in the address space of the current process{:_}")
)

type localPrincipal struct {
	security.Principal
	close func() error
}

func (p *localPrincipal) Close() error {
	return p.close()
}

func loadPrincipalLocally(credentials string) (agent.Principal, error) {
	p, err := vsecurity.LoadPersistentPrincipal(credentials, nil)
	if err != nil {
		return nil, err
	}
	agentDir := constants.AgentDir(credentials)
	credsLock := lock.NewDirLock(credentials)
	agentLock := lock.NewDirLock(agentDir)
	if err := credsLock.Lock(); err != nil {
		return nil, err
	}
	defer credsLock.Unlock()
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		return nil, err
	}
	if grabbedLock, err := agentLock.TryLock(); err != nil {
		return nil, err
	} else if !grabbedLock {
		return nil, fmt.Errorf("principal already locked by another process. Examine the contents of the lock file in %v for details", agentDir)
	}
	return &localPrincipal{Principal: p, close: agentLock.Unlock}, nil
}

// TODO(caprita): The agent is expected to be up for the entire lifetime of a
// client; however, it should be possible for the client app to survive an agent
// death by having the app restart the agent and reconnecting. The main barrier
// to making the agent restartable by clients dynamically is the requirement to
// fetch the principal decryption passphrase over stdin.  Look into using
// something like pinentry.

// LoadPrincipal loads a principal (private key, BlessingRoots, BlessingStore)
// from the provided directory using the security agent.  If an agent serving
// the principal is not present, a new one is started as a separate daemon
// process.  The new agent may use os.Stdin and os.Stdout in order to fetch a
// private key decryption passphrase.  If an agent serving the principal is not
// found and a new one cannot be started, LoadPrincipal tries to load the
// principal in the current process' address space, which will be exclusive for
// this process; if that fails too (for example, because the principal is
// encrypted), an error is returned.  The caller should call Close on the
// returned Principal once it's no longer used, in order to free up resources
// and allow the agent to terminate once it has no more clients.
func LoadPrincipal(credsDir string) (agent.Principal, error) {
	if finfo, err := os.Stat(credsDir); err != nil || err == nil && !finfo.IsDir() {
		return nil, verror.New(errNotADirectory, nil, credsDir)
	}
	sockPath := constants.SocketPath(credsDir)

	// Since we don't hold a lock between LaunchAgent and
	// NewAgentPrincipal, the agent could go away by the time we try to
	// connect to it (e.g. it decides there are no clients and exits
	// voluntarily); even if we held a lock, the agent could still crash in
	// the meantime.  Therefore, we retry a few times before giving up.
	tries, maxTries := 0, 5
	var (
		agentBin     string
		agentVersion version.T
	)
	dontStartAgent := os.Getenv(ref.EnvCredentialsNoAgent) != ""
	for {
		// If the socket exists and we're able to connect over the
		// socket to an existing agent, we're done.  This works even if
		// the client only has access to the socket but no write access
		// to the credentials dir or agent dir required in order to
		// launch an agent.
		if p, err := NewAgentPrincipal(sockPath, 0); err == nil {
			return p, nil
		} else if tries == maxTries {
			return nil, err
		}

		// If launching an agent is not in the cards, just try loading
		// the principal locally.
		if dontStartAgent {
			p, err := loadPrincipalLocally(credsDir)
			if err != nil {
				return nil, verror.New(errLoadLocally, nil, err)
			}
			return p, nil
		}

		// Go ahead and try to launch an agent.  Implicitly requires the
		// current process to have write permissions to the credentials
		// directory.
		tries++
		if agentBin == "" {
			var err error
			if agentBin, agentVersion, err = findAgent(); err != nil {
				// Try loading the principal in memory without
				// an external agent.
				p, lerr := loadPrincipalLocally(credsDir)
				if lerr != nil {
					return nil, verror.New(errFindAgent, nil, err, lerr)
				}
				vlog.Infof("Couldn't find a suitable agent binary (%v); loaded principal in the current process' address space.", err)
				return p, nil
			}
		}
		if err := launcher.LaunchAgent(
			credsDir,
			agentBin,
			false,
			fmt.Sprintf("--%s=%s", constants.TimeoutFlag, time.Minute),
			fmt.Sprintf("--%s=%v", constants.VersionFlag, agentVersion)); err != nil {
			// Try loading the principal in memory without an
			// external agent.
			// NOTE(caprita): If the agent fails to start because of
			// bad/missing decryption passphrase, retrying locally
			// is futile.  There's some finessing we can do.
			p, lerr := loadPrincipalLocally(credsDir)
			if lerr != nil {
				return nil, verror.New(errLaunchAgent, nil, err, lerr)
			}
			vlog.Infof("Couldn't launch agent (%v); loaded principal in the current process' address space.", err)
			return p, nil
		}
	}
}

const agentBinName = "v23agentd"

// findAgent tries to locate an agent binary that we can run.
func findAgent() (string, version.T, error) {
	// TODO(caprita): Besides checking PATH, we can also look under
	// JIRI_ROOT.  Also, consider caching a copy of the agent binary under
	// <creds dir>/agent?
	var defaultVersion version.T
	agentBin, err := exec.LookPath(agentBinName)
	if err != nil {
		return "", defaultVersion, fmt.Errorf("failed to find %v: %v", agentBinName, err)
	}
	// Verify that the binary we found contains a compatible agent version
	// range in its metadata.
	cmd := exec.Command(agentBin, "--metadata")
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Dir = os.TempDir()
	if err = cmd.Start(); err != nil {
		return "", defaultVersion, fmt.Errorf("failed to run %v: %v", cmd.Args, err)
	}
	waitErr := make(chan error, 1)
	go func() {
		waitErr <- cmd.Wait()
	}()
	timeout := 5 * time.Second
	select {
	case err := <-waitErr:
		if err != nil {
			return "", defaultVersion, fmt.Errorf("%v failed: %v", cmd.Args, err)
		}
	case <-time.After(timeout):
		if err := cmd.Process.Kill(); err != nil {
			vlog.Errorf("Failed to kill %v (PID:%d): %v", cmd.Args, cmd.Process.Pid, err)
		}
		if err := <-waitErr; err != nil {
			vlog.Errorf("%v failed: %v", cmd.Args, err)
		}
		return "", defaultVersion, fmt.Errorf("%v failed to complete in %v", cmd.Args, timeout)
	}
	md, err := metadata.FromXML(b.Bytes())
	if err != nil {
		return "", defaultVersion, fmt.Errorf("%v output failed to parse as XML metadata: %v", cmd.Args, err)
	}
	versionRange, err := version.RangeFromString(nil, md.Lookup("v23agentd.VersionMin"), md.Lookup("v23agentd.VersionMax"))
	if err != nil {
		return "", defaultVersion, fmt.Errorf("%v output does not contain a valid version range: %v", cmd.Args, err)
	}
	versionToUse, err := version.Common(nil, versionRange, version.Supported)
	if err != nil {
		return "", defaultVersion, fmt.Errorf("%v version incompatible: %v", cmd.Args, err)
	}
	return agentBin, versionToUse, nil
}
