// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"v.io/x/ref/services/agent/server"
)

// In addition to the socket used to serve the principal, the agent also
// maintains a separate socket where it accepts administrative commands.  The
// socket file is at <creds dir>/agent/commands, and implements the following
// simple protocol:
//
// Read: command (the name of the command, in plain text form).  Some commands
// support an optional argument of the form <command>=<arg>.  E.g. "GRACE" or
// "GRACE=1h".
//
// Write: result of executing command (in plain text form).  By convention,
// commands with no arguments do not mutate the state of the agent, while
// commands with arguments do and return "OK".
//
// The set of currently supported commands:
//
// "GRACE"
// Returns the value of the grace period (in seconds) that the agent stays up
// after its last client disconnected.
//
// "GRACE=<seconds>"
// Sets the grace period to the specified number of seconds and returns "OK".
//
// "IDLE"
// Returns how long (in seconds) the agent has been idle (no clients).
//
// "IDLE=0"
// Resets the agent's idle duration to 0, and returns "OK".
//
// "CONNECTIONS"
// Returns the number of existing client connections.
//
// "EXIT"
// Makes the agent exit, and returns "OK" (which may or may not reach the
// network layer depending on the timing of the process exit).
//
//
// The protocol is purposefully plain text, to avoid the need for a specialized
// client to administer the agent.  E.g. on linux, one can:
// Query the agent for number of clients:
// $> echo CONNECTIONS | nc -q 1 -U <creds dir>/agent/commands
//
// Instruct the agent to exit:
// $> echo EXIT | nc -q 1 -U <creds dir>/agent/commands
//
// A sample session of commands:
// $> nc -U /tmp/creds/agent/commands
// help
// Unknown command. Valid commands:
// GRACE
// IDLE
// CONNECTIONS
// EXIT
// Commands are case-sensitive.
//
// GRACE
// 60.00
// IDLE
// 24.27
// IDLE
// 28.13
// IDLE=0
// OK
// IDLE
// 4.79
// CONNECTIONS
// 0
// IDLE
// 10.72
// GRACE=10s
// Failed to parse 10s as an integer: strconv.ParseInt: parsing "10s": invalid syntax
// GRACE=10
// OK

type commandChannels struct {
	graceChange           chan time.Duration
	graceQuery, idleQuery chan chan time.Duration
	idleChange, exit      chan struct{}
}

func setupCommandHandlers(ipc server.IPCState) (handlers map[string]commandHandler, channels commandChannels) {
	channels = commandChannels{
		graceChange: make(chan time.Duration, 10),
		graceQuery:  make(chan chan time.Duration, 10),
		idleChange:  make(chan struct{}, 10),
		idleQuery:   make(chan chan time.Duration, 10),
		exit:        make(chan struct{}),
	}
	formatDuration := func(d time.Duration) string {
		return fmt.Sprintf("%.2f", d.Seconds())
	}
	handlers = make(map[string]commandHandler)
	handlers["GRACE"] = func(arg string) string {
		if arg != "" {
			nSecs, err := strconv.Atoi(arg)
			if err != nil {
				return fmt.Sprintf("Failed to parse %v as an integer: %v", arg, err)
			}
			channels.graceChange <- time.Duration(nSecs) * time.Second
			return "OK"
		}
		graceCh := make(chan time.Duration)
		channels.graceQuery <- graceCh
		d := <-graceCh
		return formatDuration(d)
	}
	handlers["IDLE"] = func(arg string) string {
		if arg == "0" {
			channels.idleChange <- struct{}{}
			return "OK"
		}
		idleCh := make(chan time.Duration)
		channels.idleQuery <- idleCh
		d := <-idleCh
		return formatDuration(d)
	}
	handlers["CONNECTIONS"] = func(string) string {
		return strconv.Itoa(ipc.NumConnections())
	}
	handlers["EXIT"] = func(string) string {
		close(channels.exit)
		return "OK"
	}
	return
}

type commandHandler func(string) string

func setupCommandSocket(agentDir string, handlers map[string]commandHandler) (func() error, error) {
	socketPath := filepath.Join(agentDir, "commands")
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	stopCh, goroutineCh := make(chan struct{}), make(chan struct{})
	go func() {
		for {
			c, err := listener.Accept()
			select {
			case <-stopCh:
				close(goroutineCh)
				return
			default:
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "Command accept error:", err)
				continue
			}
			go commandConnection(c, handlers)
		}
	}()
	return func() error {
		close(stopCh)
		if err := listener.Close(); err != nil {
			return err
		}
		timeout := 10 * time.Second
		select {
		case <-goroutineCh:
			return nil
		case <-time.After(timeout):
			return fmt.Errorf("commands listener failed to shut down ater %s", timeout)
		}
	}, nil
}

func commandConnection(c net.Conn, handlers map[string]commandHandler) {
	scanner := bufio.NewScanner(c)
	for scanner.Scan() {
		command := strings.SplitN(scanner.Text(), "=", 2)
		if len(command) == 0 {
			fmt.Println(os.Stderr, "Empty command")
			continue
		}
		arg := ""
		if len(command) == 2 {
			arg = command[1]
		}
		handler, ok := handlers[command[0]]
		if !ok {
			fmt.Fprintln(os.Stderr, "Unknown command:", command[0])
			fmt.Fprintln(c, "Unknown command. Valid commands:")
			for cmd := range handlers {
				fmt.Fprintln(c, cmd)
			}
			fmt.Fprintln(c, "Commands are case-sensitive.\n")
			continue
		}
		fmt.Fprintln(c, handler(arg))
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading command:", err)
	}
}
