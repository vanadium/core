// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"v.io/x/lib/cmdline"
	"v.io/x/lib/metadata"
	"v.io/x/ref"
	vsignals "v.io/x/ref/lib/signals"
	"v.io/x/ref/services/agent/internal/constants"
	"v.io/x/ref/services/agent/internal/launcher"
	"v.io/x/ref/services/agent/internal/lock"
	"v.io/x/ref/services/agent/internal/version"
	"v.io/x/ref/services/agent/server"
)

func init() {
	metadata.Insert("v23agentd.VersionMin", version.Supported.Min.String())
	metadata.Insert("v23agentd.VersionMax", version.Supported.Max.String())
}

var (
	versionToUse = version.Supported.Max
	idleGrace    time.Duration
	daemon       bool
	stop         bool
	credentials  string
	createCreds  bool
)

func main() {
	cmdAgentD.Flags.StringVar(&credentials, constants.CredentialsFlag, os.Getenv(ref.EnvCredentials), fmt.Sprintf("Credentials directory.  Defaults to the %s environment variable.", ref.EnvCredentials))
	cmdAgentD.Flags.BoolVar(&createCreds, "create", false, "Whether to create the credentials if missing.")
	cmdAgentD.Flags.Var(&versionToUse, constants.VersionFlag, "Version that the agent should use.  Will fail if the version is not in the range of supported versions (obtained from the --metadata flag)")
	cmdAgentD.Flags.BoolVar(&daemon, constants.DaemonFlag, true, "Run the agent as a daemon (returns right away but leaves the agent running in the background)")
	cmdAgentD.Flags.DurationVar(&idleGrace, constants.TimeoutFlag, 0, "How long the agent stays alive without any client connections. Zero implies no timeout.")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdAgentD)
}

var cmdAgentD = &cmdline.Command{
	Name:  "v23agentd",
	Short: "Holds a private key in memory and makes it available to other processes",
	Long: `
Command v23agentd manages the security agent daemon, which holds the private
key, blessings and recognized roots of a principal in memory and makes the
principal available to other processes.

Other processes can access the agent credentials when V23_AGENT_PATH is set to
<credential dir>/agent/sock.

Without arguments, v23agentd starts the agent to exclusively serve the specified
credentials.

Exits right away if another agent is already serving the credentials.
Exits when there are no processes accessing the credentials (after a grace
period).

Example:
 $ v23agentd --credentials=$HOME/.credentials
 $ V23_AGENT_PATH=$HOME/.credentials/agent/sock principal dump
`,
	Children: []*cmdline.Command{cmdStop},
}

type runner func(*cmdline.Env, []string) error

func (r runner) Run(env *cmdline.Env, args []string) error {
	if credentials == "" {
		return env.UsageErrorf("--%s must specify an existing credentials directory", constants.CredentialsFlag)
	}
	return r(env, args)
}

func init() {
	cmdAgentD.Runner = runner(runAgentD)
}

func flagsFor(cmd *cmdline.Command) (ret []string) {
	cmdAgentD.ParsedFlags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case constants.DaemonFlag:
		case constants.CredentialsFlag:
		default:
			ret = append(ret, fmt.Sprintf("--%s=%s", f.Name, f.Value))
		}
	})
	return
}

func runAgentD(env *cmdline.Env, args []string) error {
	if !version.Supported.Contains(versionToUse) {
		return fmt.Errorf("version %v not in the supported range %v", versionToUse, version.Supported)
	}
	switch _, err := os.Stat(credentials); {
	case os.IsNotExist(err):
		if createCreds {
			if err := os.MkdirAll(credentials, 0700); err != nil {
				return fmt.Errorf("failed to create credentials dir \"%s\": %v", credentials, err)
			}
		} else {
			return fmt.Errorf("credentials dir \"%s\" does not exist", credentials)
		}
	case err != nil:
		return fmt.Errorf("cannot access credentials dir \"%s\": %v", credentials, err)
	}
	if daemon {
		return launcher.LaunchAgent(credentials, os.Args[0], true, flagsFor(cmdAgentD)...)
	}

	notifyParent, detachIO, err := setupNotifyParent(env)
	if err != nil {
		return err
	}
	cleanup, commandChannels, ipc, err := initialize(env, credentials)
	switch err {
	case nil, errAlreadyRunning:
		notifyParent(constants.ServingMsg)
		if os.Getenv(constants.EnvAgentNoPrintCredsEnv) == "" {
			fmt.Fprintf(env.Stdout, "%v=%v\n", ref.EnvAgentPath, constants.SocketPath(credentials))
		}
		if err == errAlreadyRunning {
			return nil
		}
	default:
		return err
	}
	defer cleanup()
	if detachIO {
		if err := detachStdInOutErr(constants.AgentDir(credentials)); err != nil {
			return err
		}
		// TODO(caprita): Consider ignoring SIGHUP.
	}

	noConnections := make(chan struct{})
	go idleWatch(env, ipc, noConnections, commandChannels)

	select {
	case sig := <-vsignals.ShutdownOnSignals(nil):
		fmt.Fprintln(env.Stderr, "Received signal", sig)
	case <-noConnections:
		fmt.Fprintln(env.Stderr, "Idle timeout")
	case <-commandChannels.exit:
		fmt.Fprintln(env.Stderr, "Received exit command")
	}
	return nil
}

var errAlreadyRunning = errors.New("already running")

// initialize sets up the service to serve the principal.  Upon success, the
// agent lock is locked and a cleanup function is returned (which includes
// unlocking the agent lock).  Otherwise, an error is returned.
func initialize(env *cmdline.Env, credentials string) (func(), commandChannels, server.IPCState, error) {
	agentDir := constants.AgentDir(credentials)
	// Lock the credentials dir and then try to grab the agent lock.  We
	// need to first lock the credentials dir before the agent lock in order
	// to avoid the following race: the old agent is about to exit; it stops
	// serving, but before it releases the agent lock, a client tries
	// connecting to the socket and fails; the client spawns a new agent
	// that comes in and tries to grab the agent lock; it can't, and then
	// exits; the old agent then also exits; this leaves no agent running.
	credsLock := lock.NewDirLock(credentials).Must()
	agentLock := lock.NewDirLock(agentDir).Must()
	credsLock.Lock()
	cleanup := credsLock.Unlock
	// In case we return early from initialize.
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		return nil, commandChannels{}, nil, err
	}
	if !agentLock.TryLock() {
		// Another agent is already serving the credentials.
		return nil, commandChannels{}, nil, errAlreadyRunning
	}
	cleanup = push(cleanup, agentLock.Unlock)
	p, err := server.LoadPrincipal(credentials, createCreds)
	if err != nil {
		return nil, commandChannels{}, nil, fmt.Errorf("failed to create new principal from dir(%s): %v", credentials, err)
	}
	ipc, err := server.Serve(p, constants.SocketPath(credentials))
	if err != nil {
		return nil, commandChannels{}, nil, fmt.Errorf("Serve failed: %v", err)
	}
	cleanup = push(cleanup, ipc.Close)
	handlers, commandCh := setupCommandHandlers(ipc)
	closeCommands, err := setupCommandSocket(agentDir, handlers)
	if err != nil {
		return nil, commandChannels{}, nil, fmt.Errorf("setupCommandSocket failed: %v", err)
	}
	cleanup = push(cleanup, func() {
		if err := closeCommands(); err != nil {
			fmt.Fprintf(env.Stderr, "closeCommands failed: %v\n", err)
		}
	})
	cleanup = push(cleanup, credsLock.Lock)
	// Disable running cleanup at the end of initialize.
	defer func() {
		cleanup = nil
	}()
	credsLock.Unlock()
	return cleanup, commandCh, ipc, nil
}

func push(a, b func()) func() {
	return func() {
		b()
		a()
	}
}

// setupNotifyParent checks if the agent was started by a parent process that
// configured a pipe over which the agent is supposed to send a status message.
// If so, it returns a function that should be called to send the status.  It
// also returns whether the agent should detach from stdout, stderr, and stdin
// following its initialization.
func setupNotifyParent(env *cmdline.Env) (func(string), bool, error) {
	parentPipeFD := os.Getenv(constants.EnvAgentParentPipeFD)
	if parentPipeFD == "" {
		return func(string) {}, false, nil
	}
	fd, err := strconv.Atoi(parentPipeFD)
	if err != nil {
		return nil, false, err
	}
	parentPipe := os.NewFile(uintptr(fd), "parent-pipe")
	return func(message string) {
		defer parentPipe.Close()
		if n, err := fmt.Fprintln(parentPipe, message); n != len(message)+1 || err != nil {
			// No need to stop the agent if we fail to write back to
			// the parent.  The agent is otherwise healthy.
			fmt.Fprintf(env.Stderr, "Failed to write %v to parent: (%d, %v)\n", message, n, err)
		}
	}, true, nil
}

func idleWatch(env *cmdline.Env, ipc server.IPCState, noConnections chan struct{}, channels commandChannels) {
	defer close(noConnections)
	grace := idleGrace
	var idleStartOverride time.Time
	idleDuration := func() time.Duration {
		idleStart := ipc.IdleStartTime()
		if idleStart.IsZero() {
			return 0
		}
		if idleStart.Before(idleStartOverride) {
			idleStart = idleStartOverride
		}
		return time.Now().Sub(idleStart)
	}
	for {
		var sleepCh <-chan time.Time
		if grace > 0 {
			idle := idleDuration()
			if idle > grace {
				fmt.Fprintln(env.Stderr, "IDLE for", idle, "exiting.")
				return
			}
			sleepFor := grace - idle
			if sleepFor < time.Millisecond {
				sleepFor = time.Millisecond
			}
			sleepCh = time.After(sleepFor)
		}
		select {
		case <-sleepCh:
		case newGrace := <-channels.graceChange:
			grace = newGrace
		case graceReport := <-channels.graceQuery:
			graceReport <- grace
		case <-channels.idleChange:
			idleStartOverride = time.Now()
		case idleReport := <-channels.idleQuery:
			idleReport <- idleDuration()
		}
	}
}

var cmdStop = &cmdline.Command{
	Name:  "stop",
	Short: "Stops the agent",
	Long: `
If an agent serving the specified credentials is running, stops the agent.  If
none is running, does nothing.
`,
	Runner: runner(runStop),
}

func runStop(env *cmdline.Env, args []string) error {
	commandsSock := filepath.Join(constants.AgentDir(credentials), "commands")
	switch _, err := os.Stat(commandsSock); {
	case os.IsNotExist(err):
		fmt.Fprintf(env.Stdout, "No agent appears to be running for \"%s\".\n", credentials)
		return nil
	case err != nil:
		return err
	}
	cmds, err := net.Dial("unix", commandsSock)
	if err != nil {
		return err
	}
	defer cmds.Close()
	if _, err := cmds.Write([]byte("EXIT\n")); err != nil {
		return err
	}
	cmdsRead := bufio.NewScanner(cmds)
	if cmdsRead.Scan() && cmdsRead.Text() != "OK" {
		return fmt.Errorf("unexpected reply for EXIT command: %v", cmdsRead.Text())
	}
	return cmdsRead.Err()
}
