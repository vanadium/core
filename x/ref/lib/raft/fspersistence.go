// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raft

// The persistent log consists of a directory containing the files:
//    snap.<term>.<index> - snapshots of the client database to which all log entries
//                          up to <term>.<index> have been applied.
//    log.<term>.<index> - a log commencing after <term>.<index>
//
// The tail of the log is kept in memory.  Once the tail is long enough (> snapshotThreshold) a
// snapshot is taken.

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"v.io/v23/context"
	"v.io/x/lib/vlog"
)

const (
	defaultSnapshotThreshold = 10 * 1024 * 1024
)

// fsPersist is persistent state implemented in a file system.  It stores the state in a single directory.
// One file, <directory>/persistent, represents the persistent non-log state and a series of files,
// <directory>/log<n>, the logs.  A new log file is created on startup and after every snapshot.
type fsPersist struct {
	sync.Mutex

	r           *raft
	currentTerm Term
	votedFor    string

	dir       string // the directory
	client    RaftClient
	baseIndex Index // The index before the log starts (either 0 or from a snapshot).
	baseTerm  Term  // The term before the log starts (either 0 or from a snapshot).
	lastIndex Index // last index stored to
	lastTerm  Term  // term for entry at lastIindex

	// For appending the log file.
	lf      *os.File
	encoder *gob.Encoder

	// In-memory version of the log file.  We depend on log truncation to keep this
	// to a reasonable size.
	logTail           map[Index]*logEntry
	firstLogTailIndex Index
	snapshotThreshold int64

	// Name of last snapshot read or written.
	lastSnapFile  string
	lastSnapTerm  Term
	lastSnapIndex Index
	snapping      bool // true if we are in the process of creating a snapshot.
}

// ControlEntry's are appended to the log to reflect changes in the current term and/or the voted for
// leader during ellections
type ControlEntry struct {
	InUse       bool
	CurrentTerm Term
	VotedFor    string
}

// SetCurrentTerm implements persistent.SetCurrentTerm.
func (p *fsPersist) SetCurrentTerm(ct Term) error {
	p.Lock()
	defer p.Unlock()
	p.currentTerm = ct
	return p.syncLog()
}

// IncCurrentTerm increments the current term.
func (p *fsPersist) IncCurrentTerm() error {
	p.Lock()
	defer p.Unlock()
	p.currentTerm++
	return p.syncLog()
}

// SetVotedFor implements persistent.SetVotedFor.
func (p *fsPersist) SetVotedFor(vf string) error {
	p.Lock()
	defer p.Unlock()
	if vf == p.votedFor {
		return nil
	}
	p.votedFor = vf
	return p.syncLog()
}

// SetVotedFor implements persistent.SetCurrentTermAndVotedFor.
func (p *fsPersist) SetCurrentTermAndVotedFor(ct Term, vf string) error {
	p.Lock()
	defer p.Unlock()
	p.currentTerm = ct
	p.votedFor = vf
	return p.syncLog()
}

// CurrentTerm implements persistent.CurrentTerm.
func (p *fsPersist) CurrentTerm() Term {
	p.Lock()
	defer p.Unlock()
	return p.currentTerm
}

// LastIndex implements persistent.LastIndex.
func (p *fsPersist) LastIndex() Index {
	p.Lock()
	defer p.Unlock()
	return p.lastIndex
}

// LastTerm implements persistent.LastTerm.
func (p *fsPersist) LastTerm() Term {
	p.Lock()
	defer p.Unlock()
	return p.lastTerm
}

// VotedFor implements persistent.VotedFor.
func (p *fsPersist) VotedFor() string {
	p.Lock()
	defer p.Unlock()
	return p.votedFor
}

// Close implements persistent.Close.
func (p *fsPersist) Close() {
	p.lf.Sync() //nolint:errcheck
	p.lf.Close()
}

// AppendToLog implements persistent.AppendToLog.
func (p *fsPersist) AppendToLog(ctx *context.T, prevTerm Term, prevIndex Index, entries []LogEntry) error {
	p.Lock()
	defer p.Unlock()

	if prevIndex != p.baseIndex || prevTerm != p.baseTerm {
		// We will not log if the previous entry either doesn't exist or has the wrong term.
		le := p.lookup(prevIndex)
		if le == nil {
			return ErrorfOutOfSequence(ctx, "append %v, %v out of sequence", prevTerm, prevIndex)
		} else if le.Term != prevTerm {
			return ErrorfOutOfSequence(ctx, "append %v, %v out of sequence", prevTerm, prevIndex)
		}
	}
	for i, e := range entries {
		// If its already in the log, do nothing.
		le, ok := p.logTail[prevIndex+Index(i)+1]
		if ok && le.Term == e.Term {
			continue
		}

		// Add both to the log logTail and the log file.
		// TODO(p): Think about syncing the output before returning.
		ne := e
		if err := p.encoder.Encode(ne); err != nil {
			return fmt.Errorf("append %d, %d, %v: %s", prevTerm, prevIndex, ne, err)
		}
		p.addToLogTail(&ne)
	}

	return nil
}

// ConsiderSnapshot implements Persistent.ConsiderSnapshot.
func (p *fsPersist) ConsiderSnapshot(ctx *context.T, lastAppliedTerm Term, lastAppliedIndex Index) {
	p.Lock()
	if p.snapping {
		p.Unlock()
		return
	}

	// Take a snapshot if the log is too big.
	if int64(lastAppliedIndex-p.firstLogTailIndex) < p.snapshotThreshold {
		p.Unlock()
		return
	}
	p.snapping = true
	p.Unlock()

	safeToProceed := make(chan struct{})
	//nolint:errcheck
	go p.takeSnapshot(ctx, lastAppliedTerm, lastAppliedIndex, safeToProceed)
	<-safeToProceed
}

// TrimLog trims the log to contain only entries from prevIndex on.  This will fail if prevIndex isn't
// in the logTail or if creating or writing the new log file fails.
func (p *fsPersist) trimLog(ctx *context.T, prevTerm Term, prevIndex Index) error {
	// Nothing to trim?
	vlog.Infof("trimLog(%d, %d)", prevTerm, prevIndex)
	if prevIndex == 0 {
		return nil
	}

	p.Lock()
	le := p.lookup(prevIndex)
	p.Unlock()
	if le == nil {
		return ErrorfOutOfSequence(ctx, "append %v, %v out of sequence", 0, prevIndex)
	}

	// Create a new log file.
	fn, encoder, err := p.newLog(prevTerm, prevIndex)
	if err != nil {
		return err
	}

	// Append all log entries after prevIndex.  Unlock while writing so we don't
	// overly affect performance.  We might miss some entries that are added while the
	// lock is released so redo this loop later under lock.
	i := 1
	for {
		p.Lock()
		le, ok := p.logTail[prevIndex+Index(i)]
		p.Unlock()
		if !ok {
			break
		}
		if err := encoder.Encode(*le); err != nil {
			os.Remove(fn)
			return fmt.Errorf("trim %d, %v: %s", prevIndex, *le, err)
		}
		i++
	}

	// Finish up under lock.
	p.Lock()
	defer p.Unlock()
	for {
		le, ok := p.logTail[prevIndex+Index(i)]
		if !ok {
			break
		}
		if err := encoder.Encode(*le); err != nil {
			os.Remove(fn)
			return fmt.Errorf("trim %d, %v: %s", prevIndex, *le, err)
		}
		i++
	}

	// Start using new file.
	p.encoder = encoder
	p.syncLog() //nolint:errcheck

	// Remove some logTail entries.  Try to keep at least half of the entries around in case
	// we'll need them to update a lagging member.
	if prevIndex >= p.firstLogTailIndex {
		i := p.firstLogTailIndex + (prevIndex-p.firstLogTailIndex)/2
		for p.firstLogTailIndex <= i {
			delete(p.logTail, p.firstLogTailIndex)
			p.firstLogTailIndex++
		}
	}

	return nil
}

// takeSnapshot is a go routine that starts a snapshot and on success trims the log.
// 'safeToProceed' is closed when it is safe to continue applying commands.
func (p *fsPersist) takeSnapshot(ctx *context.T, t Term, i Index, safeToProceed chan struct{}) error {
	r := p.r
	defer func() { p.Lock(); p.snapping = false; p.Unlock() }()

	// Create a file for the snapshot.  Lock to prevent any logs from being applied while we do this.
	fn := p.snapPath(termIndexToFileName(t, i))
	vlog.Infof("snapshot %s", fn)
	fp, err := os.OpenFile(fn, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		close(safeToProceed)
		vlog.Errorf("snapshot %s: %s", fn, err)
		return err
	}

	// Start the snapshot.
	c := make(chan error)
	if err := r.client.SaveToSnapshot(ctx, fp, c); err != nil {
		close(safeToProceed)
		fp.Close()
		os.Remove(fn)
		vlog.Errorf("snapshot %s: %s", fn, err)
		return err
	}

	// At this point the client has saved whatever it needs for the snapshot (copy on write perhaps)
	// so we can release the raft server to continue applying commands.
	close(safeToProceed)

	// Wait for the snapshot to finish.
	if err := <-c; err != nil {
		fp.Close()
		os.Remove(fn)
		vlog.Errorf("snapshot %s: %s", fn, err)
		return err
	}
	if err := fp.Sync(); err != nil {
		os.Remove(fn)
		vlog.Errorf("snapshot %s: %s", fn, err)
		return err
	}
	if err := fp.Close(); err != nil {
		os.Remove(fn)
		vlog.Errorf("snapshot %s: %s", fn, err)
		return err
	}

	if err := p.trimLog(ctx, t, i); err != nil {
		os.Remove(fn)
		vlog.Errorf("snapshot %s: %s", fn, err)
		return err
	}

	p.Lock()
	p.lastSnapFile, p.lastSnapTerm, p.lastSnapIndex = fn, t, i
	p.Unlock()

	vlog.Infof("snapshot %s succeeded", fn)
	return nil
}

// SnapshotFromLeader implements Persistence.SnapshotFromLeader.  Called with p.r locked.
func (p *fsPersist) SnapshotFromLeader(ctx *context.T, term Term, index Index, call raftProtoInstallSnapshotServerCall) error {
	r := p.r

	// First securely save the snapshot.
	fn := p.snapPath(termIndexToFileName(term, index))
	fp, err := os.OpenFile(fn, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	rstream := call.RecvStream()
	for rstream.Advance() {
		value := rstream.Value()
		if _, err := fp.Write(value); err != nil {
			fp.Close()
			os.Remove(fn)
			return err
		}
	}
	if err := fp.Sync(); err != nil {
		fp.Close()
		os.Remove(fn)
		return err
	}
	if err := fp.Close(); err != nil {
		os.Remove(fn)
		return err
	}
	if err := rstream.Err(); err != nil {
		os.Remove(fn)
		return err
	}

	// Now try restoring client state with it.
	fp, err = os.Open(fn)
	if err != nil {
		return err
	}
	defer fp.Close()
	if err := r.client.RestoreFromSnapshot(ctx, index, fp); err != nil {
		return err
	}
	r.applied.term, r.applied.index = term, index
	p.Lock()
	p.baseTerm, p.baseIndex = term, index
	p.lastTerm, p.lastIndex = term, index
	p.Unlock()
	return nil
}

func (p *fsPersist) lookup(i Index) *logEntry {
	if le, ok := p.logTail[i]; ok {
		return le
	}
	return nil
}

// Lookup implements persistent.Lookup.
func (p *fsPersist) Lookup(i Index) *logEntry {
	p.Lock()
	defer p.Unlock()
	return p.lookup(i)
}

// LookupPrevious implements persistent.LookupPrevious.
func (p *fsPersist) LookupPrevious(i Index) (Term, Index, bool) {
	p.Lock()
	defer p.Unlock()
	i--
	if i == p.baseIndex {
		return p.baseTerm, i, true
	}
	le := p.lookup(i)
	if le == nil {
		return 0, i, false
	}
	return le.Term, i, true
}

// syncLog assumes that p is locked.
func (p *fsPersist) syncLog() error {
	ce := ControlEntry{InUse: true, CurrentTerm: p.currentTerm, VotedFor: p.votedFor}
	if err := p.encoder.Encode(ce); err != nil {
		return fmt.Errorf("syncLog: %s", err)
	}
	p.lf.Sync() //nolint:errcheck
	return nil
}

type Entry struct {
	LogEntry
	ControlEntry
}

// readLog reads a log file into memory.
//
// Assumes p is locked.
func (p *fsPersist) readLog(file string) error {
	vlog.Infof("reading %s", file)
	f, err := os.OpenFile(file, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	d := gob.NewDecoder(f)
	for {
		var e Entry
		if err := d.Decode(&e); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if e.InUse {
			p.currentTerm = e.CurrentTerm
			p.votedFor = e.VotedFor
		}
		if e.Index != 0 && (p.lastIndex == 0 || e.Index <= p.lastIndex+1) {
			p.addToLogTail(&LogEntry{Term: e.Term, Index: e.Index, Cmd: e.Cmd})
		}
	}
}

// addToLogTail adds the entry to the logTail if not already there and returns true
// if the logTail changed.
func (p *fsPersist) addToLogTail(wle *LogEntry) bool {
	le := &logEntry{Term: wle.Term, Index: wle.Index, Cmd: wle.Cmd, Type: wle.Type}
	ole, ok := p.logTail[le.Index]
	if ok {
		if ole.Term == le.Term {
			return false
		}
		// Remove all higher entries.
		for i := le.Index + 1; i <= p.lastIndex; i++ {
			delete(p.logTail, i)
		}
	}
	p.logTail[le.Index] = le
	p.lastIndex = le.Index
	p.lastTerm = le.Term
	return true
}

// openPersist returns an object that implements the persistent interface.  If the directory doesn't
// already exist, it is created.  If there is a problem with the contained files, an error is returned.
func openPersist(ctx *context.T, r *raft, snapshotThreshold int64) (*fsPersist, error) {
	if snapshotThreshold == 0 {
		snapshotThreshold = defaultSnapshotThreshold
	}
	p := &fsPersist{r: r, dir: r.logDir, logTail: make(map[Index]*logEntry), client: r.client, snapshotThreshold: snapshotThreshold}
	p.Lock()
	defer p.Unlock()

	// Randomize max size so all members aren't checkpointing at the same time.
	p.snapshotThreshold += rand.Int63n(1 + (p.snapshotThreshold >> 3))

	// Read the persistent state, the latest snapshot, and any log entries since then.
	if err := p.readState(ctx, r); err != nil {
		if err := p.createState(); err != nil {
			return nil, err
		}
	}

	// Start a new command log.
	if err := p.rotateLog(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *fsPersist) snapPath(s string) string {
	return path.Join(p.dir, s+".snap")
}

func (p *fsPersist) logPath(s string) string {
	return path.Join(p.dir, s+".log")
}

func termIndexToFileName(t Term, i Index) string {
	return fmt.Sprintf("%20.20d.%20.20d", t, i)
}

// paerseFileName returns the term and index from a file name of the form <term>.<index>[.whatever]
func parseFileName(s string) (Term, Index, error) {
	p := strings.Split(path.Base(s), ".")
	if len(p) < 2 {
		return 0, 0, errors.New("not log name")
	}
	t, err := strconv.ParseInt(p[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	if err != nil {
		return 0, 0, err
	}
	i, err := strconv.ParseInt(p[1], 10, 64)
	return Term(t), Index(i), err
}

// readState is called on start up.  State is recreated from the last valid snapshot and
// any subsequent log files.  If any log file has a decoding error, we give up and return
// an error.
//
// TODO(p): This is perhaps too extreme since we should be able to continue from a snapshot
// and a prefix of the log.
//
// Assumes p is locked.
func (p *fsPersist) readState(ctx *context.T, r *raft) error { //nolint:gocyclo
	d, err := os.Open(p.dir)
	if err != nil {
		return err
	}
	defer d.Close()

	// Find all snapshot and log files.
	vlog.Infof("reading directory %s", p.dir)
	files, err := d.Readdirnames(0)
	if err != nil {
		return err
	}
	lmap := make(map[string]struct{})
	var logs []string
	smap := make(map[string]struct{})
	for _, f := range files {
		if strings.HasSuffix(f, ".log") {
			f = strings.TrimSuffix(f, ".log")
			lmap[f] = struct{}{}
			logs = append(logs, f)
		} else if strings.HasSuffix(f, ".snap") {
			smap[strings.TrimSuffix(f, ".snap")] = struct{}{}
		}
	}

	// Throw out any snapshots that don't have an equivalent log file; they are not complete.
	for s := range smap {
		if _, ok := lmap[s]; !ok {
			os.Remove(p.snapPath(s))
			delete(smap, s)
		}
	}

	// Find the latest readable snapshot.
	var snaps []string
	for s := range smap {
		snaps = append(snaps, s)
	}
	sort.StringSlice(snaps).Sort()
	sort.StringSlice(logs).Sort()
	firstLog := termIndexToFileName(0, 0)
	for i := len(snaps) - 1; i >= 0; i-- {
		f := p.snapPath(snaps[i])
		fp, err := os.Open(f)
		if err != nil {
			os.Remove(f)
			continue
		}
		term, index, err := parseFileName(snaps[i])
		if err != nil {
			os.Remove(f)
			continue
		}
		vlog.Infof("restoring from snapshot %s", f)
		err = p.client.RestoreFromSnapshot(ctx, index, fp)
		vlog.Infof("restored from snapshot %s", err)
		fp.Close()
		if err == nil {
			// The snapshot was readable, we're done.
			firstLog = snaps[i]
			// The name of the snapshot has the last applied entry
			// in that snapshot.  Remember it so that we won't reapply
			// any log entries in case they aren't equipotent.
			r.applied.term, r.applied.index = term, index
			p.baseTerm, p.baseIndex = term, index
			p.lastTerm, p.lastIndex = term, index
			p.lastSnapFile, p.lastSnapTerm, p.lastSnapIndex = f, term, index
			break
		}
		os.Remove(f)
	}

	// find the log that goes with the snapshot.
	found := false
	for _, l := range logs {
		if l == firstLog {
			found = true
		}
		f := p.logPath(l)
		if !found {
			// Remove stale log files.
			os.Remove(f)
			continue
		}
		// Give up on first bad/incomplete log entry.  If we lost any log entries,
		// we should be refreshed from some other member.
		if err := p.readLog(f); err != nil {
			vlog.Infof("reading %s: %s", f, err)
			break
		}
	}
	r.setMatchIndex(r.me, p.lastIndex)

	return nil
}

func (p *fsPersist) OpenLatestSnapshot(ctx *context.T) (io.Reader, Term, Index, error) {
	p.Lock()
	if len(p.lastSnapFile) == 0 {
		p.Unlock()
		return nil, 0, 0, errors.New("no snapshot")
	}
	fn, t, i := p.lastSnapFile, p.lastSnapTerm, p.lastSnapIndex
	p.Unlock()
	fp, err := os.Open(fn)
	if err != nil {
		return nil, 0, 0, err
	}
	return fp, t, i, nil
}

// newLog starts a new log file. It doesn't point the p.encoder at it yet.
func (p *fsPersist) newLog(prevTerm Term, prevIndex Index) (string, *gob.Encoder, error) {
	// Create an append only encoder for log entries.
	fn := p.logPath(termIndexToFileName(prevTerm, prevIndex))
	append, err := os.OpenFile(fn, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return "", nil, err
	}
	vlog.Infof("new log file %s", fn)
	return fn, gob.NewEncoder(append), nil
}

// rotateLog starts a new log.
//
// Assumes p is already locked.
func (p *fsPersist) rotateLog() error {
	_, encoder, err := p.newLog(p.lastTerm, p.lastIndex)
	if err != nil {
		return err
	}
	p.encoder = encoder
	p.syncLog() //nolint:errcheck
	return nil
}

// createState throws out all state and starts again from scratch.
func (p *fsPersist) createState() error {
	os.RemoveAll(p.dir)
	if err := os.Mkdir(p.dir, 0770); err != nil {
		return err
	}
	return nil
}
