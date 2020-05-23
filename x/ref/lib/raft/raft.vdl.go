// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: raft

//nolint:golint
package raft

import (
	"io"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/vdl"
)

var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

//////////////////////////////////////////////////
// Type definitions

// Term is a counter incremented each time a member starts an election.  The log will
// show gaps in Term numbers because all elections need not be successful.
type Term uint64

func (Term) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/raft.Term"`
}) {
}

func (x Term) VDLIsZero() bool { //nolint:gocyclo
	return x == 0
}

func (x Term) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueUint(vdlTypeUint641, uint64(x)); err != nil {
		return err
	}
	return nil
}

func (x *Term) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	switch value, err := dec.ReadValueUint(64); {
	case err != nil:
		return err
	default:
		*x = Term(value)
	}
	return nil
}

// Index is an index into the log.  The log entries are numbered sequentially.  At the moment
// the entries RaftClient.Apply()ed should be sequential but that will change if we introduce
// system entries. For example, we could have an entry type that is used to add members to the
// set of replicas.
type Index uint64

func (Index) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/raft.Index"`
}) {
}

func (x Index) VDLIsZero() bool { //nolint:gocyclo
	return x == 0
}

func (x Index) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueUint(vdlTypeUint642, uint64(x)); err != nil {
		return err
	}
	return nil
}

func (x *Index) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	switch value, err := dec.ReadValueUint(64); {
	case err != nil:
		return err
	default:
		*x = Index(value)
	}
	return nil
}

// The LogEntry is what the log consists of.  'error' starts nil and is never written to stable
// storage.  It represents the result of RaftClient.Apply(Cmd, Index).  This is a hack but I
// haven't figured out a better way.
type LogEntry struct {
	Term  Term
	Index Index
	Cmd   []byte
	Type  byte
}

func (LogEntry) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/raft.LogEntry"`
}) {
}

func (x LogEntry) VDLIsZero() bool { //nolint:gocyclo
	if x.Term != 0 {
		return false
	}
	if x.Index != 0 {
		return false
	}
	if len(x.Cmd) != 0 {
		return false
	}
	if x.Type != 0 {
		return false
	}
	return true
}

func (x LogEntry) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct3); err != nil {
		return err
	}
	if x.Term != 0 {
		if err := enc.NextFieldValueUint(0, vdlTypeUint641, uint64(x.Term)); err != nil {
			return err
		}
	}
	if x.Index != 0 {
		if err := enc.NextFieldValueUint(1, vdlTypeUint642, uint64(x.Index)); err != nil {
			return err
		}
	}
	if len(x.Cmd) != 0 {
		if err := enc.NextFieldValueBytes(2, vdlTypeList4, x.Cmd); err != nil {
			return err
		}
	}
	if x.Type != 0 {
		if err := enc.NextFieldValueUint(3, vdl.ByteType, uint64(x.Type)); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *LogEntry) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = LogEntry{}
	if err := dec.StartValue(vdlTypeStruct3); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != vdlTypeStruct3 {
			index = vdlTypeStruct3.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueUint(64); {
			case err != nil:
				return err
			default:
				x.Term = Term(value)
			}
		case 1:
			switch value, err := dec.ReadValueUint(64); {
			case err != nil:
				return err
			default:
				x.Index = Index(value)
			}
		case 2:
			if err := dec.ReadValueBytes(-1, &x.Cmd); err != nil {
				return err
			}
		case 3:
			switch value, err := dec.ReadValueUint(8); {
			case err != nil:
				return err
			default:
				x.Type = byte(value)
			}
		}
	}
}

//////////////////////////////////////////////////
// Const definitions

const ClientEntry = byte(0)
const RaftEntry = byte(1)

//////////////////////////////////////////////////
// Interface definitions

// raftProtoClientMethods is the client interface
// containing raftProto methods.
//
// raftProto is used by the members of a raft set to communicate with each other.
type raftProtoClientMethods interface {
	// Members returns the current set of ids of raft members.
	Members(*context.T, ...rpc.CallOpt) ([]string, error)
	// Leader returns the id of the current leader.
	Leader(*context.T, ...rpc.CallOpt) (string, error)
	// RequestVote starts a new round of voting.  It returns the server's current Term and true if
	// the server voted for the client.
	RequestVote(_ *context.T, term Term, candidateId string, lastLogTerm Term, lastLogIndex Index, _ ...rpc.CallOpt) (Term Term, Granted bool, _ error)
	// AppendToLog is sent by the leader to tell followers to append an entry.  If cmds
	// is empty, this is a keep alive message (at a random interval after a keep alive, followers
	// will initiate a new round of voting).
	//   term -- the current term of the sender
	//   leader -- the id of the sender
	//   prevIndex -- the index of the log entry immediately preceding cmds
	//   prevTerm -- the term of the log entry immediately preceding cmds.  The receiver must have
	//               received the previous index'd entry and it must have had the same term.  Otherwise
	//               an error is returned.
	//   leaderCommit -- the index of the last committed entry, i.e., the one a quorum has guaranteed
	//                   to have logged.
	//   cmds -- sequential log entries starting at prevIndex+1
	AppendToLog(_ *context.T, term Term, leader string, prevIndex Index, prevTerm Term, leaderCommit Index, cmds []LogEntry, _ ...rpc.CallOpt) error
	// Append is sent to the leader by followers.  Only the leader is allowed to send AppendToLog.
	// If a follower receives an Append() call it performs an Append() to the leader to run the actual
	// Raft algorithm.  The leader will respond after it has RaftClient.Apply()ed the command.
	//
	// Returns the term and index of the append entry or an error.
	Append(_ *context.T, cmd []byte, _ ...rpc.CallOpt) (term Term, index Index, _ error)
	// Committed returns the commit index of the leader.
	Committed(*context.T, ...rpc.CallOpt) (index Index, _ error)
	// InstallSnapshot is sent from the leader to follower to install the given snapshot.  It is
	// sent when it becomes apparent that the leader does not have log entries needed by the follower
	// to progress.  'term' and 'index' represent the last LogEntry RaftClient.Apply()ed to the
	// snapshot.
	InstallSnapshot(_ *context.T, term Term, leader string, appliedTerm Term, appliedIndex Index, _ ...rpc.CallOpt) (raftProtoInstallSnapshotClientCall, error)
}

// raftProtoClientStub adds universal methods to raftProtoClientMethods.
type raftProtoClientStub interface {
	raftProtoClientMethods
	rpc.UniversalServiceMethods
}

// raftProtoClient returns a client stub for raftProto.
func raftProtoClient(name string) raftProtoClientStub {
	return implraftProtoClientStub{name}
}

type implraftProtoClientStub struct {
	name string
}

func (c implraftProtoClientStub) Members(ctx *context.T, opts ...rpc.CallOpt) (o0 []string, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Members", nil, []interface{}{&o0}, opts...)
	return
}

func (c implraftProtoClientStub) Leader(ctx *context.T, opts ...rpc.CallOpt) (o0 string, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Leader", nil, []interface{}{&o0}, opts...)
	return
}

func (c implraftProtoClientStub) RequestVote(ctx *context.T, i0 Term, i1 string, i2 Term, i3 Index, opts ...rpc.CallOpt) (o0 Term, o1 bool, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "RequestVote", []interface{}{i0, i1, i2, i3}, []interface{}{&o0, &o1}, opts...)
	return
}

func (c implraftProtoClientStub) AppendToLog(ctx *context.T, i0 Term, i1 string, i2 Index, i3 Term, i4 Index, i5 []LogEntry, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "AppendToLog", []interface{}{i0, i1, i2, i3, i4, i5}, nil, opts...)
	return
}

func (c implraftProtoClientStub) Append(ctx *context.T, i0 []byte, opts ...rpc.CallOpt) (o0 Term, o1 Index, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Append", []interface{}{i0}, []interface{}{&o0, &o1}, opts...)
	return
}

func (c implraftProtoClientStub) Committed(ctx *context.T, opts ...rpc.CallOpt) (o0 Index, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Committed", nil, []interface{}{&o0}, opts...)
	return
}

func (c implraftProtoClientStub) InstallSnapshot(ctx *context.T, i0 Term, i1 string, i2 Term, i3 Index, opts ...rpc.CallOpt) (ocall raftProtoInstallSnapshotClientCall, err error) {
	var call rpc.ClientCall
	if call, err = v23.GetClient(ctx).StartCall(ctx, c.name, "InstallSnapshot", []interface{}{i0, i1, i2, i3}, opts...); err != nil {
		return
	}
	ocall = &implraftProtoInstallSnapshotClientCall{ClientCall: call}
	return
}

// raftProtoInstallSnapshotClientStream is the client stream for raftProto.InstallSnapshot.
type raftProtoInstallSnapshotClientStream interface {
	// SendStream returns the send side of the raftProto.InstallSnapshot client stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors
		// encountered while sending, or if Send is called after Close or
		// the stream has been canceled.  Blocks if there is no buffer
		// space; will unblock when buffer space is available or after
		// the stream has been canceled.
		Send(item []byte) error
		// Close indicates to the server that no more items will be sent;
		// server Recv calls will receive io.EOF after all sent items.
		// This is an optional call - e.g. a client might call Close if it
		// needs to continue receiving items from the server after it's
		// done sending.  Returns errors encountered while closing, or if
		// Close is called after the stream has been canceled.  Like Send,
		// blocks if there is no buffer space available.
		Close() error
	}
}

// raftProtoInstallSnapshotClientCall represents the call returned from raftProto.InstallSnapshot.
type raftProtoInstallSnapshotClientCall interface {
	raftProtoInstallSnapshotClientStream
	// Finish performs the equivalent of SendStream().Close, then blocks until
	// the server is done, and returns the positional return values for the call.
	//
	// Finish returns immediately if the call has been canceled; depending on the
	// timing the output could either be an error signaling cancelation, or the
	// valid positional return values from the server.
	//
	// Calling Finish is mandatory for releasing stream resources, unless the call
	// has been canceled or any of the other methods return an error.  Finish should
	// be called at most once.
	Finish() error
}

type implraftProtoInstallSnapshotClientCall struct {
	rpc.ClientCall
}

func (c *implraftProtoInstallSnapshotClientCall) SendStream() interface {
	Send(item []byte) error
	Close() error
} {
	return implraftProtoInstallSnapshotClientCallSend{c}
}

type implraftProtoInstallSnapshotClientCallSend struct {
	c *implraftProtoInstallSnapshotClientCall
}

func (c implraftProtoInstallSnapshotClientCallSend) Send(item []byte) error {
	return c.c.Send(item)
}
func (c implraftProtoInstallSnapshotClientCallSend) Close() error {
	return c.c.CloseSend()
}
func (c *implraftProtoInstallSnapshotClientCall) Finish() (err error) {
	err = c.ClientCall.Finish()
	return
}

// raftProtoServerMethods is the interface a server writer
// implements for raftProto.
//
// raftProto is used by the members of a raft set to communicate with each other.
type raftProtoServerMethods interface {
	// Members returns the current set of ids of raft members.
	Members(*context.T, rpc.ServerCall) ([]string, error)
	// Leader returns the id of the current leader.
	Leader(*context.T, rpc.ServerCall) (string, error)
	// RequestVote starts a new round of voting.  It returns the server's current Term and true if
	// the server voted for the client.
	RequestVote(_ *context.T, _ rpc.ServerCall, term Term, candidateId string, lastLogTerm Term, lastLogIndex Index) (Term Term, Granted bool, _ error)
	// AppendToLog is sent by the leader to tell followers to append an entry.  If cmds
	// is empty, this is a keep alive message (at a random interval after a keep alive, followers
	// will initiate a new round of voting).
	//   term -- the current term of the sender
	//   leader -- the id of the sender
	//   prevIndex -- the index of the log entry immediately preceding cmds
	//   prevTerm -- the term of the log entry immediately preceding cmds.  The receiver must have
	//               received the previous index'd entry and it must have had the same term.  Otherwise
	//               an error is returned.
	//   leaderCommit -- the index of the last committed entry, i.e., the one a quorum has guaranteed
	//                   to have logged.
	//   cmds -- sequential log entries starting at prevIndex+1
	AppendToLog(_ *context.T, _ rpc.ServerCall, term Term, leader string, prevIndex Index, prevTerm Term, leaderCommit Index, cmds []LogEntry) error
	// Append is sent to the leader by followers.  Only the leader is allowed to send AppendToLog.
	// If a follower receives an Append() call it performs an Append() to the leader to run the actual
	// Raft algorithm.  The leader will respond after it has RaftClient.Apply()ed the command.
	//
	// Returns the term and index of the append entry or an error.
	Append(_ *context.T, _ rpc.ServerCall, cmd []byte) (term Term, index Index, _ error)
	// Committed returns the commit index of the leader.
	Committed(*context.T, rpc.ServerCall) (index Index, _ error)
	// InstallSnapshot is sent from the leader to follower to install the given snapshot.  It is
	// sent when it becomes apparent that the leader does not have log entries needed by the follower
	// to progress.  'term' and 'index' represent the last LogEntry RaftClient.Apply()ed to the
	// snapshot.
	InstallSnapshot(_ *context.T, _ raftProtoInstallSnapshotServerCall, term Term, leader string, appliedTerm Term, appliedIndex Index) error
}

// raftProtoServerStubMethods is the server interface containing
// raftProto methods, as expected by rpc.Server.
// The only difference between this interface and raftProtoServerMethods
// is the streaming methods.
type raftProtoServerStubMethods interface {
	// Members returns the current set of ids of raft members.
	Members(*context.T, rpc.ServerCall) ([]string, error)
	// Leader returns the id of the current leader.
	Leader(*context.T, rpc.ServerCall) (string, error)
	// RequestVote starts a new round of voting.  It returns the server's current Term and true if
	// the server voted for the client.
	RequestVote(_ *context.T, _ rpc.ServerCall, term Term, candidateId string, lastLogTerm Term, lastLogIndex Index) (Term Term, Granted bool, _ error)
	// AppendToLog is sent by the leader to tell followers to append an entry.  If cmds
	// is empty, this is a keep alive message (at a random interval after a keep alive, followers
	// will initiate a new round of voting).
	//   term -- the current term of the sender
	//   leader -- the id of the sender
	//   prevIndex -- the index of the log entry immediately preceding cmds
	//   prevTerm -- the term of the log entry immediately preceding cmds.  The receiver must have
	//               received the previous index'd entry and it must have had the same term.  Otherwise
	//               an error is returned.
	//   leaderCommit -- the index of the last committed entry, i.e., the one a quorum has guaranteed
	//                   to have logged.
	//   cmds -- sequential log entries starting at prevIndex+1
	AppendToLog(_ *context.T, _ rpc.ServerCall, term Term, leader string, prevIndex Index, prevTerm Term, leaderCommit Index, cmds []LogEntry) error
	// Append is sent to the leader by followers.  Only the leader is allowed to send AppendToLog.
	// If a follower receives an Append() call it performs an Append() to the leader to run the actual
	// Raft algorithm.  The leader will respond after it has RaftClient.Apply()ed the command.
	//
	// Returns the term and index of the append entry or an error.
	Append(_ *context.T, _ rpc.ServerCall, cmd []byte) (term Term, index Index, _ error)
	// Committed returns the commit index of the leader.
	Committed(*context.T, rpc.ServerCall) (index Index, _ error)
	// InstallSnapshot is sent from the leader to follower to install the given snapshot.  It is
	// sent when it becomes apparent that the leader does not have log entries needed by the follower
	// to progress.  'term' and 'index' represent the last LogEntry RaftClient.Apply()ed to the
	// snapshot.
	InstallSnapshot(_ *context.T, _ *raftProtoInstallSnapshotServerCallStub, term Term, leader string, appliedTerm Term, appliedIndex Index) error
}

// raftProtoServerStub adds universal methods to raftProtoServerStubMethods.
type raftProtoServerStub interface {
	raftProtoServerStubMethods
	// DescribeInterfaces the raftProto interfaces.
	Describe__() []rpc.InterfaceDesc
}

// raftProtoServer returns a server stub for raftProto.
// It converts an implementation of raftProtoServerMethods into
// an object that may be used by rpc.Server.
func raftProtoServer(impl raftProtoServerMethods) raftProtoServerStub {
	stub := implraftProtoServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := rpc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := rpc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implraftProtoServerStub struct {
	impl raftProtoServerMethods
	gs   *rpc.GlobState
}

func (s implraftProtoServerStub) Members(ctx *context.T, call rpc.ServerCall) ([]string, error) {
	return s.impl.Members(ctx, call)
}

func (s implraftProtoServerStub) Leader(ctx *context.T, call rpc.ServerCall) (string, error) {
	return s.impl.Leader(ctx, call)
}

func (s implraftProtoServerStub) RequestVote(ctx *context.T, call rpc.ServerCall, i0 Term, i1 string, i2 Term, i3 Index) (Term, bool, error) {
	return s.impl.RequestVote(ctx, call, i0, i1, i2, i3)
}

func (s implraftProtoServerStub) AppendToLog(ctx *context.T, call rpc.ServerCall, i0 Term, i1 string, i2 Index, i3 Term, i4 Index, i5 []LogEntry) error {
	return s.impl.AppendToLog(ctx, call, i0, i1, i2, i3, i4, i5)
}

func (s implraftProtoServerStub) Append(ctx *context.T, call rpc.ServerCall, i0 []byte) (Term, Index, error) {
	return s.impl.Append(ctx, call, i0)
}

func (s implraftProtoServerStub) Committed(ctx *context.T, call rpc.ServerCall) (Index, error) {
	return s.impl.Committed(ctx, call)
}

func (s implraftProtoServerStub) InstallSnapshot(ctx *context.T, call *raftProtoInstallSnapshotServerCallStub, i0 Term, i1 string, i2 Term, i3 Index) error {
	return s.impl.InstallSnapshot(ctx, call, i0, i1, i2, i3)
}

func (s implraftProtoServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implraftProtoServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{raftProtoDesc}
}

// raftProtoDesc describes the raftProto interface.
var raftProtoDesc rpc.InterfaceDesc = descraftProto

// descraftProto hides the desc to keep godoc clean.
var descraftProto = rpc.InterfaceDesc{
	Name:    "raftProto",
	PkgPath: "v.io/x/ref/lib/raft",
	Doc:     "// raftProto is used by the members of a raft set to communicate with each other.",
	Methods: []rpc.MethodDesc{
		{
			Name: "Members",
			Doc:  "// Members returns the current set of ids of raft members.",
			OutArgs: []rpc.ArgDesc{
				{Name: "", Doc: ``}, // []string
			},
		},
		{
			Name: "Leader",
			Doc:  "// Leader returns the id of the current leader.",
			OutArgs: []rpc.ArgDesc{
				{Name: "", Doc: ``}, // string
			},
		},
		{
			Name: "RequestVote",
			Doc:  "// RequestVote starts a new round of voting.  It returns the server's current Term and true if\n// the server voted for the client.",
			InArgs: []rpc.ArgDesc{
				{Name: "term", Doc: ``},         // Term
				{Name: "candidateId", Doc: ``},  // string
				{Name: "lastLogTerm", Doc: ``},  // Term
				{Name: "lastLogIndex", Doc: ``}, // Index
			},
			OutArgs: []rpc.ArgDesc{
				{Name: "Term", Doc: ``},    // Term
				{Name: "Granted", Doc: ``}, // bool
			},
		},
		{
			Name: "AppendToLog",
			Doc:  "// AppendToLog is sent by the leader to tell followers to append an entry.  If cmds\n// is empty, this is a keep alive message (at a random interval after a keep alive, followers\n// will initiate a new round of voting).\n//   term -- the current term of the sender\n//   leader -- the id of the sender\n//   prevIndex -- the index of the log entry immediately preceding cmds\n//   prevTerm -- the term of the log entry immediately preceding cmds.  The receiver must have\n//               received the previous index'd entry and it must have had the same term.  Otherwise\n//               an error is returned.\n//   leaderCommit -- the index of the last committed entry, i.e., the one a quorum has guaranteed\n//                   to have logged.\n//   cmds -- sequential log entries starting at prevIndex+1",
			InArgs: []rpc.ArgDesc{
				{Name: "term", Doc: ``},         // Term
				{Name: "leader", Doc: ``},       // string
				{Name: "prevIndex", Doc: ``},    // Index
				{Name: "prevTerm", Doc: ``},     // Term
				{Name: "leaderCommit", Doc: ``}, // Index
				{Name: "cmds", Doc: ``},         // []LogEntry
			},
		},
		{
			Name: "Append",
			Doc:  "// Append is sent to the leader by followers.  Only the leader is allowed to send AppendToLog.\n// If a follower receives an Append() call it performs an Append() to the leader to run the actual\n// Raft algorithm.  The leader will respond after it has RaftClient.Apply()ed the command.\n//\n// Returns the term and index of the append entry or an error.",
			InArgs: []rpc.ArgDesc{
				{Name: "cmd", Doc: ``}, // []byte
			},
			OutArgs: []rpc.ArgDesc{
				{Name: "term", Doc: ``},  // Term
				{Name: "index", Doc: ``}, // Index
			},
		},
		{
			Name: "Committed",
			Doc:  "// Committed returns the commit index of the leader.",
			OutArgs: []rpc.ArgDesc{
				{Name: "index", Doc: ``}, // Index
			},
		},
		{
			Name: "InstallSnapshot",
			Doc:  "// InstallSnapshot is sent from the leader to follower to install the given snapshot.  It is\n// sent when it becomes apparent that the leader does not have log entries needed by the follower\n// to progress.  'term' and 'index' represent the last LogEntry RaftClient.Apply()ed to the\n// snapshot.",
			InArgs: []rpc.ArgDesc{
				{Name: "term", Doc: ``},         // Term
				{Name: "leader", Doc: ``},       // string
				{Name: "appliedTerm", Doc: ``},  // Term
				{Name: "appliedIndex", Doc: ``}, // Index
			},
		},
	},
}

// raftProtoInstallSnapshotServerStream is the server stream for raftProto.InstallSnapshot.
type raftProtoInstallSnapshotServerStream interface {
	// RecvStream returns the receiver side of the raftProto.InstallSnapshot server stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() []byte
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	}
}

// raftProtoInstallSnapshotServerCall represents the context passed to raftProto.InstallSnapshot.
type raftProtoInstallSnapshotServerCall interface {
	rpc.ServerCall
	raftProtoInstallSnapshotServerStream
}

// raftProtoInstallSnapshotServerCallStub is a wrapper that converts rpc.StreamServerCall into
// a typesafe stub that implements raftProtoInstallSnapshotServerCall.
type raftProtoInstallSnapshotServerCallStub struct {
	rpc.StreamServerCall
	valRecv []byte
	errRecv error
}

// Init initializes raftProtoInstallSnapshotServerCallStub from rpc.StreamServerCall.
func (s *raftProtoInstallSnapshotServerCallStub) Init(call rpc.StreamServerCall) {
	s.StreamServerCall = call
}

// RecvStream returns the receiver side of the raftProto.InstallSnapshot server stream.
func (s *raftProtoInstallSnapshotServerCallStub) RecvStream() interface {
	Advance() bool
	Value() []byte
	Err() error
} {
	return implraftProtoInstallSnapshotServerCallRecv{s}
}

type implraftProtoInstallSnapshotServerCallRecv struct {
	s *raftProtoInstallSnapshotServerCallStub
}

func (s implraftProtoInstallSnapshotServerCallRecv) Advance() bool {
	s.s.errRecv = s.s.Recv(&s.s.valRecv)
	return s.s.errRecv == nil
}
func (s implraftProtoInstallSnapshotServerCallRecv) Value() []byte {
	return s.s.valRecv
}
func (s implraftProtoInstallSnapshotServerCallRecv) Err() error {
	if s.s.errRecv == io.EOF {
		return nil
	}
	return s.s.errRecv
}

// Hold type definitions in package-level variables, for better performance.
//nolint:unused
var (
	vdlTypeUint641 *vdl.Type
	vdlTypeUint642 *vdl.Type
	vdlTypeStruct3 *vdl.Type
	vdlTypeList4   *vdl.Type
)

var initializeVDLCalled bool

// initializeVDL performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = initializeVDL()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func initializeVDL() struct{} {
	if initializeVDLCalled {
		return struct{}{}
	}
	initializeVDLCalled = true

	// Register types.
	vdl.Register((*Term)(nil))
	vdl.Register((*Index)(nil))
	vdl.Register((*LogEntry)(nil))

	// Initialize type definitions.
	vdlTypeUint641 = vdl.TypeOf((*Term)(nil))
	vdlTypeUint642 = vdl.TypeOf((*Index)(nil))
	vdlTypeStruct3 = vdl.TypeOf((*LogEntry)(nil)).Elem()
	vdlTypeList4 = vdl.TypeOf((*[]byte)(nil))

	return struct{}{}
}
