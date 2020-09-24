// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raft

import (
	"io"

	"v.io/v23/context"
)

// persistent represents all the persistent state for the raft algorithm.  The division exists should we
// want to replace the file system based store with something else.
type persistent interface {
	// AppendLog appends cmds to the log starting at Index prevIndex+1.  It will return
	// an error if there does not exist a previous command at index preIndex with term
	// prevTerm.  It will also return an error if we cannot write to the persistent store.
	AppendToLog(ctx *context.T, prevTerm Term, prevIndex Index, entries []LogEntry) error

	// SetCurrentTerm, SetVotedFor, and SetCurrentTermAndVotedFor changes the non-log persistent state.
	SetCurrentTerm(Term) error
	IncCurrentTerm() error
	SetVotedFor(id string) error
	SetCurrentTermAndVotedFor(term Term, id string) error

	// CurrentTerm returns the current Term.
	CurrentTerm() Term

	// LastIndex returns the highest index in the log.
	LastIndex() Index

	// LastTerm returns the highest term in the log.
	LastTerm() Term

	// GetVotedFor returns the current voted for string.
	VotedFor() string

	// Close all state.  No other calls can be made on this object following
	// the Close().
	Close()

	// Lookup returns the log entry at that index or nil if none exists.
	Lookup(Index) *logEntry

	// LookupPrevious returns the index and term preceding Index.  It returns false if there is none.
	// This is used when appending entries to the log.  The leader needs to send the follower the
	// term and index of the last entry appended to the log, and the follower has to check if it
	// matches what they have at that point.  However, either one may no longer have that entry, the
	// leader because he is continuing after restoring from a snapshot or the follower because he has
	// trimmed the log after a snapshot or for either when this is the first log record.  I could
	// have entered fake log entries but this seemed easier since it is closer to the logic from the
	// raft paper.
	//
	// Granted I could avoid returning the previous index and have each client do the index-1.  I just
	// felt like doing it once rather than on every call.
	LookupPrevious(Index) (Term, Index, bool)

	// ConsiderSnapshot checks to see if it is time for a snapshot and, if so, calls back the client to
	// generate a snapshot and then trims the log.  On return it is safe to continue
	// RaftClient.Apply()ing commands although the snapshot may still be in progress.
	ConsiderSnapshot(ctx *context.T, lastAppliedTerm Term, lastAppliedIndex Index)

	// SnapshotFromLeader receives and stores a snapshot from the leader and then restores the
	// client state from the snapshot.  'lastTermApplied' and 'lastIndexApplied' represent the last
	// log entry RaftClient.Apply()ed before the snapshot was taken.
	SnapshotFromLeader(ctx *context.T, lastTermApplied Term, lastIndexApplied Index, call raftProtoInstallSnapshotServerCall) error

	// OpenLatestSnapshot opens the latest snapshot and returns a reader for it.  The returned Term
	// and Index represent the last log entry RaftClient.Appy()ed before the snapshot was taken.
	OpenLatestSnapshot(ctx *context.T) (io.Reader, Term, Index, error)
}
