// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raft

import (
	"io"
	"time"

	"v.io/v23/context"
	"v.io/v23/security/access"
)

// Raft provides a consistent log across multiple instances of a client.

// RaftClient defines the call backs from the Raft library to the application.
//nolint:golint // API change required.
type RaftClient interface {
	// Apply appies a logged command, 'cmd', to the client. The commands will
	// be delivered in the same order and with the same 'index' to all clients.
	// 'index' is a monotonically increasing number and is just an index into the
	// common log.
	//
	// Whenever a client restarts (after a crash perhaps) or falls too far behind
	// (as in a partitioned network) it will be reinitialized with a RestoreFomSnapshot
	// and then replayed all subsequent logged commands.
	//
	// A client that wishes to may return empty snapshots, i.e., just close the error
	// channel without writing anything and worry about reliably storing its database
	// itself.  It that case it must remember the highest index it has seen if it wishes
	// to avoid replays.  Hence the index is supplied with the Apply().
	Apply(cmd []byte, index Index) error

	// SaveToSnapshot requests the application to write a snapshot to 'wr'.
	// Until SaveToSnapshot returns, no commands will be Apply()ed.  Closing
	// the response channel signals that the snapshot is finished.  Any
	// error written to the response channel will be logged by the library
	// and the library will discard the snapshot if any error is returned.
	SaveToSnapshot(ctx *context.T, wr io.Writer, response chan<- error) error

	// RestoreFromSnapshot requests the application to rebuild its database from the snapshot
	// it must read from 'rd'.  'index' is the last index applied to the snapshot.  No Apply()s
	// will be performed until RestoreFromSnapshot() returns. 'index' can be ignored
	// or used for debugging.
	RestoreFromSnapshot(ctx *context.T, index Index, rd io.Reader) error
}

const (
	RoleCandidate = iota // Requesting to be voted leader.
	RoleFollower
	RoleLeader
	RoleStopped
)

type Raft interface {
	// AddMember adds a new member to the server set.  "id" is actually a network address for the member,
	// currently host:port.  This has to be done before starting the server.
	AddMember(ctx *context.T, id string) error

	// ID returns the id of this member.
	ID() string

	// Start starts the local server communicating with other members.
	Start()

	// Stop terminates the server.   It cannot be Start'ed again.
	Stop()

	// Append appends a new command to the replicated log.  The command will be Apply()ed at each member
	// once a quorum has logged it. The Append() will terminate once a quorum has logged it and at least
	// the leader has Apply()ed the command.  'applyError' is the error returned by the Apply() while
	// 'raftError' is returned by the raft library itself reporting that the Append could not be
	// performed.
	Append(ctx *context.T, cmd []byte) (applyError, raftError error)

	// Status returns the state of the raft.
	Status() (myID string, role int, leader string)

	// StartElection forces an election.  Normally just used for debugging.
	StartElection()
}

// RaftConfig is passed to NewRaft to avoid lots of parameters.
//nolint:golint // API change required.
type RaftConfig struct {
	LogDir            string            // Directory in which to put log and snapshot files.
	HostPort          string            // For RPCs from other members.
	ServerName        string            // Where to mount if not empty.
	Heartbeat         time.Duration     // Time between heartbeats.
	SnapshotThreshold int64             // Approximate number of log entries between snapshots.
	ACL               access.AccessList // For sending RPC to the members.
}

// NewRaft creates a new raft server.
func NewRaft(ctx *context.T, config *RaftConfig, client RaftClient) (Raft, error) {
	return newRaft(ctx, config, client)
}
