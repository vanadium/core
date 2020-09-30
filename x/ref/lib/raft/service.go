// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raft

import (
	"fmt"
	"reflect"

	"v.io/x/lib/vlog"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
)

// service is the implementation of the raftProto.  It is only used between members
// to communicate with each other.
type service struct {
	r          *raft
	server     rpc.Server
	serverName string
	acl        access.AccessList
}

// newService creates a new service for the raft protocol and returns its network endpoints.
func (s *service) newService(ctx *context.T, r *raft, serverName, hostPort string, acl access.AccessList) ([]naming.Endpoint, error) {
	var err error
	s.r = r
	s.serverName = serverName
	s.acl = acl
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Addrs: rpc.ListenAddrs{{"tcp", hostPort}}})
	_, s.server, err = v23.WithNewDispatchingServer(ctx, serverName, s)
	if err != nil {
		return nil, err
	}
	return s.server.Status().Endpoints, nil
}

// Lookup implements rpc.Dispatcher.Lookup.
func (s *service) Lookup(ctx *context.T, name string) (interface{}, security.Authorizer, error) {
	return raftProtoServer(s), s, nil
}

// Authorize allows anyone matching the ACL from the RaftConfig or with the same public
// key as the server.
func (s *service) Authorize(ctx *context.T, call security.Call) error {
	if names, _ := security.RemoteBlessingNames(ctx, call); len(names) != 0 {
		if s.acl.Includes(names...) {
			return nil
		}
	}
	if l, r := call.LocalBlessings().PublicKey(), call.RemoteBlessings().PublicKey(); l != nil && reflect.DeepEqual(l, r) {
		return nil
	}
	return verror.ErrNoAccess.Errorf(ctx, "access denied")
}

// Members implements raftProto.Members.
func (s *service) Members(ctx *context.T, call rpc.ServerCall) ([]string, error) {
	r := s.r
	r.Lock()
	defer r.Unlock()

	var members []string
	for m := range r.memberMap {
		members = append(members, m)
	}
	return members, nil
}

// Members implements raftProto.Leader.
func (s *service) Leader(ctx *context.T, call rpc.ServerCall) (string, error) {
	r := s.r
	r.Lock()
	defer r.Unlock()
	return r.leader, nil
}

// RequestVote implements raftProto.RequestVote.
func (s *service) RequestVote(ctx *context.T, call rpc.ServerCall, term Term, candidate string, lastLogTerm Term, lastLogIndex Index) (Term, bool, error) {
	r := s.r

	// The voting needs to be atomic.
	r.Lock()
	defer r.Unlock()

	// An old election?
	if term < r.p.CurrentTerm() {
		return r.p.CurrentTerm(), false, nil
	}

	// If the term is higher than the current election term, then we are into a new election.
	if term > r.p.CurrentTerm() {
		r.setRoleAndWatchdogTimer(RoleFollower)
		r.p.SetCurrentTermAndVotedFor(term, "") //nolint:errcheck
	}
	vf := r.p.VotedFor()

	// Have we already voted for someone else (for example myself)?
	if vf != "" && vf != candidate {
		return r.p.CurrentTerm(), false, nil
	}

	// If we have a more up to date log, ignore the candidate. (RAFT's safety restriction)
	if r.p.LastTerm() > lastLogTerm {
		return r.p.CurrentTerm(), false, nil
	}
	if r.p.LastTerm() == lastLogTerm && r.p.LastIndex() > lastLogIndex {
		return r.p.CurrentTerm(), false, nil
	}

	// Vote for candidate and make sure we're a follower.
	r.setRole(RoleFollower)
	r.p.SetCurrentTermAndVotedFor(term, candidate) //nolint:errcheck
	return r.p.CurrentTerm(), true, nil
}

// AppendToLog implements RaftProto.AppendToLog.
func (s *service) AppendToLog(ctx *context.T, call rpc.ServerCall, term Term, leader string, prevIndex Index, prevTerm Term, leaderCommit Index, entries []LogEntry) error {
	r := s.r

	// The append needs to be atomic.
	r.Lock()

	// The leader has to be at least as up to date as we are.
	if term < r.p.CurrentTerm() {
		r.Unlock()
		return fmt.Errorf("new term %v < %v", term, r.p.CurrentTerm())
	}

	// At this point we  accept the sender as leader and become a follower.
	if r.leader != leader {
		vlog.VI(2).Infof("@%s new leader %s during AppendToLog", r.me.id, leader)
	}
	r.leader = leader
	r.lcv.Broadcast()

	// Update our term if we are behind.
	if term > r.p.CurrentTerm() {
		r.p.SetCurrentTermAndVotedFor(term, "") //nolint:errcheck
	}

	// Restart our timer since we just heard from the leader.
	r.setRoleAndWatchdogTimer(RoleFollower)

	// Append if we can.
	if err := r.p.AppendToLog(ctx, prevTerm, prevIndex, entries); err != nil {
		vlog.Errorf("@%s AppendToLog returns %s", r.me.id, err)
		r.Unlock()
		return err
	}
	r.Unlock()
	r.newCommit <- leaderCommit
	return nil
}

// Append implements RaftProto.Append.
func (s *service) Append(ctx *context.T, call rpc.ServerCall, cmd []byte) (Term, Index, error) {
	r := s.r
	r.Lock()

	if r.role != RoleLeader {
		r.Unlock()
		return 0, 0, ErrorfNotLeader(ctx, "not the leader")
	}

	// Assign an index and term to the log entry.
	le := LogEntry{Term: r.p.CurrentTerm(), Index: r.p.LastIndex() + 1, Cmd: cmd}

	// Append to our own log.
	if err := r.p.AppendToLog(ctx, r.p.LastTerm(), r.p.LastIndex(), []LogEntry{le}); err != nil {
		// This shouldn't happen.
		r.Unlock()
		return 0, 0, err
	}

	// Update the fact that we've logged it.
	r.setMatchIndex(r.me, le.Index)

	// Tell each follower to update.
	r.kickFollowers()
	r.Unlock()

	// The entry is not yet committed or applied.  The caller must verify that itself.
	return le.Term, le.Index, nil
}

// InstallSnapshot implements RaftProto.InstallSnapshot.
func (s *service) InstallSnapshot(ctx *context.T, call raftProtoInstallSnapshotServerCall, term Term, leader string, appliedTerm Term, appliedIndex Index) error {
	r := s.r

	// The snapshot needs to be atomic.
	r.Lock()
	defer r.Unlock()

	// The leader has to be at least as up to date as we are.
	if term < r.p.CurrentTerm() {
		return fmt.Errorf("new term %v < %v", term, r.p.CurrentTerm())
	}

	// At this point we  accept the sender as leader and become a follower.
	if r.leader != leader {
		vlog.VI(2).Infof("@%s new leader %s during InstallSnapshot", r.me.id, leader)
	}
	r.leader = leader
	r.lcv.Broadcast()

	// Update our term if we are behind.
	if term > r.p.CurrentTerm() {
		r.p.SetCurrentTermAndVotedFor(term, "") //nolint:errcheck
	}

	// Restart our timer since we just heard from the leader.
	r.setRoleAndWatchdogTimer(RoleFollower)

	// Store the snapshot and restore client from it.
	return r.p.SnapshotFromLeader(ctx, appliedTerm, appliedIndex, call)
}

func (s *service) Committed(ctx *context.T, call rpc.ServerCall) (Index, error) {
	r := s.r
	r.Lock()
	defer r.Unlock()
	if r.role != RoleLeader {
		return 0, ErrorfNotLeader(ctx, "not the leader")
	}
	return r.commitIndex, nil
}
