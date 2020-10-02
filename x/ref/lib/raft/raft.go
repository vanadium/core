// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raft

// This package implements the Raft protocol, https://ramcloud.stanford.edu/raft.pdf. The
// logged commands are strings.   If someone wishes a more complex command structure, they
// should use an encoding (e.g. json) into the strings.

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"sync"
	"time"

	"v.io/x/lib/vlog"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
)

// member keeps track of another member's state.
type member struct {
	id         string
	nextIndex  Index         // Next log index to send to this follower.
	matchIndex Index         // Last entry logged by this follower.
	stopped    chan struct{} // Follower go routine closes this to indicate it has terminated.
	update     chan struct{}
	timer      *time.Timer
}

// memberSlice is used for sorting members by highest logged (matched) entry.
type memberSlice []*member

func (m memberSlice) Len() int           { return len(m) }
func (m memberSlice) Less(i, j int) bool { return m[i].matchIndex > m[j].matchIndex }
func (m memberSlice) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// raft is the implementation of the raft library.
type raft struct {
	sync.Mutex

	ctx       *context.T
	cancel    context.CancelFunc
	rng       *rand.Rand
	timer     *time.Timer
	heartbeat time.Duration

	// rpc interface between instances.
	s service

	// Client interface.
	client RaftClient

	// applied is the highest log entry applied to the client.
	applied struct {
		index Index
		term  Term
	}

	// Raft algorithm volatile state.
	role        int
	leader      string
	quorum      int                // Number of members that form a quorum.
	commitIndex Index              // Highest index committed.
	memberMap   map[string]*member // Map of raft members (including current).
	memberSet   memberSlice        // Slice of raft members (including current).
	me          *member

	// Raft algorithm persistent state
	p      persistent
	logDir string

	// stop and stopped are for clean shutdown.  All long lived go routines (perFollower and serverEvents)
	// exit when stop is closed.  Each perFollower then closes member.stopped and serverEvents closes
	// stopped to signal that they are finished.
	stop    chan struct{} // perFollower and serverEvents go routines exit when this is closed.
	stopped chan struct{} // serverEvents go routine closes this to indicate it has terminated.

	// Each time a perFollower successfully logs entries on a follower it writes to newMatch to get the
	// serverEvents routine to possibly update the commitIndex.
	newMatch chan struct{} // Each time a follower reports a logged entry a message is sent on this.

	// Each time a follower receives a new commit index, it sends it to newCommit to get the serverEvents
	// routine to apply any newly committed entries.
	newCommit chan Index // Each received leader commit index is written to this.

	// Wait here for commitIndex to change.
	ccv *sync.Cond

	// Wait here for leadership to change.
	lcv *sync.Cond

	// Variables for the sync loop.
	sync struct {
		sync.Mutex
		requested   uint64 // Incremented each sync request.
		requestedcv *sync.Cond
		done        uint64 // Updated to last request prior to the current sync.
		donecv      *sync.Cond
		stopped     chan struct{}
	}
}

// logentry is the in memory structure for each logged item.  It is
type logEntry struct {
	Term       Term
	Index      Index
	Cmd        []byte
	Type       byte
	ApplyError error
}

// newRaft creates a new raft server.
//  logDir        - the name of the directory in which to persist the log.
//  serverName    - a name for the server to announce itself as in a mount table.  All members should use the
//                 same name and hence be alternatives for callers.
//  hostPort      - the network address of the server
//  hb            - the interval between heartbeats.  0 means use default.
//  snapshotThreshold - the size the log can reach before we create a snapshot.  0 means use default.
//  client        - callbacks to the client.
func newRaft(ctx *context.T, config *RaftConfig, client RaftClient) (*raft, error) {
	nctx, cancel := context.WithCancel(ctx)
	r := &raft{}
	r.ctx = nctx
	r.cancel = cancel
	r.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	r.heartbeat = config.Heartbeat
	if r.heartbeat == 0 {
		r.heartbeat = 3 * time.Second
	}

	// Client interface.
	r.client = client
	r.applied.term = 0
	r.applied.index = 0

	// Raft volatile state.
	r.role = RoleStopped
	r.commitIndex = 0
	r.leader = ""
	r.memberMap = make(map[string]*member)
	r.memberSet = make([]*member, 0)
	r.AddMember(ctx, config.HostPort) //nolint:errcheck
	r.me = r.memberMap[config.HostPort]

	// Raft persistent state.
	var err error
	r.logDir = config.LogDir
	if r.p, err = openPersist(ctx, r, config.SnapshotThreshold); err != nil {
		return nil, err
	}

	// Internal communication/synchronization.
	r.newMatch = make(chan struct{}, 100)
	r.newCommit = make(chan Index, 100)
	r.ccv = sync.NewCond(r)
	r.lcv = sync.NewCond(r)
	r.sync.donecv = sync.NewCond(&r.sync)
	r.sync.requestedcv = sync.NewCond(&r.sync)

	// The RPC interface to other members.
	eps, err := r.s.newService(nctx, r, config.ServerName, config.HostPort, config.ACL)
	if err != nil {
		return nil, err
	}

	// If we're in the V namespace, just use the name as our Id.  If not create one
	// from the network address.
	r.me.id = config.ServerName
	if r.me.id == "" {
		r.me.id = getShortName(eps[0])
	}

	return r, nil
}

// getShortName will return a /host:port name if possible.  Otherwise it will just return the name
// version of the endpoint.
func getShortName(ep naming.Endpoint) string {
	if ep.Addr().Network() != "tcp" {
		return ep.Name()
	}
	return naming.JoinAddressName(ep.Addr().String(), "")
}

// AddMember adds the id as a raft member.  The id must be a vanadium name.
func (r *raft) AddMember(ctx *context.T, id string) error {
	if r.role != RoleStopped {
		// Already started.
		// TODO(p): Raft has a protocol for changing membership after
		// start.  I'll add that after I get at least one client
		// working.
		return fmt.Errorf("adding member after start")
	}
	m := &member{id: id, stopped: make(chan struct{}), update: make(chan struct{}, 10)}
	r.memberMap[id] = m
	r.memberSet = append(r.memberSet, m)
	// Quorum has to be more than half the servers.
	r.quorum = (len(r.memberSet) + 1) / 2
	return nil
}

// ID returns the vanadium name of this server.
func (r *raft) ID() string {
	return r.me.id
}

// Start gets the protocol going.
func (r *raft) Start() {
	vlog.Infof("@%s starting", r.me.id)
	r.Lock()
	defer r.Unlock()
	if r.role != RoleStopped {
		// already started
		return
	}
	r.timer = time.NewTimer(2 * r.heartbeat)

	// serverEvents serializes events for this server.
	r.stop = make(chan struct{})
	r.stopped = make(chan struct{})
	go r.serverEvents()

	// syncLoop syncs with the leader when needed.
	r.sync.stopped = make(chan struct{})
	go r.syncLoop()

	// perFollowers updates the followers when we're the leader.
	for _, m := range r.memberSet {
		if m.id != r.me.id {
			go r.perFollower(m)
		}
	}
}

// Stop ceases all function as a raft server.
func (r *raft) Stop() {
	vlog.Infof("@%s stopping", r.me.id)
	r.Lock()
	if r.role == RoleStopped {
		r.Unlock()
		r.cancel() // in case *r never got out of RoleStopped
		return
	}
	r.role = RoleStopped
	r.Unlock()
	r.cancel()

	// Stop the associated go routines.
	close(r.stop)

	// Wait for serverEvents to stop.
	<-r.stopped

	// Wait for syncLoop to stop.
	r.sync.donecv.Broadcast()
	r.sync.requestedcv.Broadcast()
	<-r.sync.stopped

	// Wait for all the perFollower routines to stop.
	for _, m := range r.memberSet {
		if m.id != r.me.id {
			<-m.stopped
		}
	}

	// Shut down the log file.
	r.p.Close()

	vlog.Infof("@%s stopping service", r.me.id)
	<-r.s.server.Closed()
	vlog.Infof("@%s stopped", r.me.id)
}

// setRoleAndWatchdogTimer called with r.l locked.
func (r *raft) setRoleAndWatchdogTimer(role int) {
	vlog.VI(2).Infof("@%s %s->%s", r.me.id, roleToString(r.role), roleToString(role))
	r.role = role
	switch role {
	case RoleFollower:
		// Wake up any RaftProto.Append()s waiting for a commitment.  They
		// will now have to give up since we are no longer leader.
		r.ccv.Broadcast()
		// Set a timer to start an election if we no longer hear from the leader.
		r.resetTimerFuzzy(2 * r.heartbeat)
	case RoleLeader:
		r.leader = r.me.id

		// Set known follower status to default values.
		for _, m := range r.memberSet {
			if m.id != r.me.id {
				m.nextIndex = r.p.LastIndex() + 1
				m.matchIndex = 0
			}
		}

		// Set my match index to the last one logged.
		r.setMatchIndex(r.me, r.p.LastIndex())

		// Let waiters know a new leader exists.
		r.lcv.Broadcast()
	case RoleCandidate:
		// If this goes off, we lost an election and need to start a new one.
		// We make it longer than the follower timeout because we make have
		// lost due to safety so give someone else a chance.
		r.resetTimerFuzzy(4 * r.heartbeat)
	}
}

// setRole called with r.l locked.
func (r *raft) setRole(role int) {
	vlog.VI(2).Infof("@%s %s->%s", r.me.id, roleToString(r.role), roleToString(role))
	r.role = role
}

func (r *raft) appendNull() {
	// Assign an index and term to the log entry.
	le := LogEntry{Term: r.p.CurrentTerm(), Index: r.p.LastIndex() + 1, Cmd: nil, Type: RaftEntry}

	// Append to our own log.
	if err := r.p.AppendToLog(r.ctx, r.p.LastTerm(), r.p.LastIndex(), []LogEntry{le}); err != nil {
		// This shouldn't happen.
		return
	}

	// Update the fact that we've logged it.
	r.setMatchIndex(r.me, le.Index)
	r.kickFollowers()
}

// Status returns the current member's id, its raft role, and who it thinks is leader.
func (r *raft) Status() (string, int, string) {
	r.Lock()
	defer r.Unlock()
	return r.me.id, r.role, r.leader
}

// StartElection starts a new round of voting.  We do this by incrementing the
// Term and, in parallel, calling each other member to vote.  If we receive a
// majority we win and send a heartbeat to each member.
//
// Someone else many get elected in the middle of the vote so we have to
// make sure we're still a candidate at the end of the voting.
func (r *raft) StartElection() {
	r.Lock()
	defer r.Unlock()
	r.startElection()
}

func (r *raft) startElection() {
	// If we can't get a response in 2 seconds, something is really wrong.
	ctx, cancel := context.WithTimeout(r.ctx, 2*time.Second)
	defer cancel()
	if err := r.p.IncCurrentTerm(); err != nil {
		// If this fails, there's no way to recover.
		vlog.Fatalf("incrementing current term: %s", err)
		return
	}
	vlog.Infof("@%s startElection new term %d", r.me.id, r.p.CurrentTerm())

	msg := []interface{}{
		r.p.CurrentTerm(),
		r.me.id,
		r.p.LastTerm(),
		r.p.LastIndex(),
	}
	var members []string
	for k, m := range r.memberMap {
		if m.id == r.me.id {
			continue
		}
		members = append(members, k)
	}
	r.setRoleAndWatchdogTimer(RoleCandidate)
	r.p.SetVotedFor(r.me.id) //nolint:errcheck
	r.leader = ""
	r.Unlock()

	// We have to do this outside the lock or the system will deadlock when two members start overlapping votes.
	type reply struct {
		term Term
		ok   bool
	}
	c := make(chan reply)
	for _, id := range members {
		go func(id string) {
			var rep reply
			client := v23.GetClient(ctx)
			if err := client.Call(ctx, id, "RequestVote", msg, []interface{}{&rep.term, &rep.ok}, options.Preresolved{}); err != nil {
				vlog.Infof("@%s sending RequestVote to %s: %s", r.me.id, id, err)
			}
			c <- rep
		}(id)
	}

	// Wait till all the voters have voted or timed out.
	oks := 1 // We vote for ourselves.
	highest := Term(0)
	for range members {
		rep := <-c
		if rep.ok {
			oks++
		}
		if rep.term > highest {
			highest = rep.term
		}
	}

	r.Lock()
	// We have to check the role since someone else may have become the leader during the round and
	// made us a follower.
	if oks <= len(members)/2 || r.role != RoleCandidate {
		if highest > r.p.CurrentTerm() {
			// If someone answered with a higher term, stop being a candidate.
			r.setRoleAndWatchdogTimer(RoleFollower)
			r.p.SetCurrentTerm(highest) //nolint:errcheck
		}
		vlog.VI(2).Infof("@%s lost election with %d votes", r.me.id, oks)
		return
	}
	vlog.Infof("@%s won election with %d votes", r.me.id, oks)
	r.setRoleAndWatchdogTimer(RoleLeader)

	// Tell followers we are now the leader.
	r.appendNull()

}

// applyCommits applies any committed entries.
func (r *raft) applyCommits(commitIndex Index) {
	for r.applied.index < commitIndex {
		// This is the only go routine that changes r.applied
		// so we don't have to protect our reads.
		next := r.applied.index + 1
		le := r.p.Lookup(next)
		if le == nil {
			// Commit index is ahead of our highest entry.
			return
		}
		switch le.Type {
		case ClientEntry:
			le.ApplyError = r.client.Apply(le.Cmd, le.Index)
		case RaftEntry:
		}

		// But we do have to lock our writes.
		r.Lock()
		r.applied.index = next
		r.applied.term = le.Term
		r.Unlock()
	}

	r.p.ConsiderSnapshot(r.ctx, r.applied.term, r.applied.index)
}

func (r *raft) lastApplied() Index {
	r.Lock()
	defer r.Unlock()
	return r.applied.index
}

func (r *raft) resetTimerFuzzy(d time.Duration) {
	fuzz := time.Duration(rand.Int63n(int64(r.heartbeat)))
	r.timer.Reset(d + fuzz)
}

func highestFromChan(i Index, c chan Index) Index {
	for {
		select {
		case j := <-c:
			if j > i {
				i = j
			}
		default:
			return i
		}
	}
}

// serverEvents is a go routine that serializes server events.  This loop performs:
// (1) all changes to commitIndex both as a leader and a follower.
// (2) all application of committed log commands.
// (3) all elections.
func (r *raft) serverEvents() {
	r.Lock()
	r.setRoleAndWatchdogTimer(RoleFollower)
	r.Unlock()
	for {
		select {
		case <-r.stop:
			// Terminate.
			close(r.stopped)
			return
		case <-r.timer.C:
			// Start an election whenever either:
			// (1) a follower hasn't heard from the leader in a random interval > 2 * heartbeat.
			// (2) a candidate hasn't won an election or been told anyone else has after hearbeat.
			r.Lock()
			switch r.role {
			case RoleCandidate:
				r.startElection()
			case RoleFollower:
				r.startElection()
			}
			r.Unlock()
		case <-r.newMatch:
			// Soak up any queued requests.
			emptyChan(r.newMatch)

			// This happens whenever we have gotten a reply from a follower.  We do it
			// here rather than in perFollower solely as a matter of taste.
			// Update the commitIndex if needed and apply any newly committed entries.
			r.Lock()
			if r.role != RoleLeader {
				r.Unlock()
				continue
			}
			sort.Sort(r.memberSet)
			ci := r.memberSet[r.quorum-1].matchIndex
			if ci <= r.commitIndex {
				r.Unlock()
				continue
			}
			r.commitIndex = ci
			r.Unlock()
			r.applyCommits(ci)
			r.ccv.Broadcast()
			r.kickFollowers()
		case i := <-r.newCommit:
			// Get highest queued up commit.
			i = highestFromChan(i, r.newCommit)

			// Update the commitIndex if needed and apply any newly committed entries.
			r.Lock()
			if r.role != RoleFollower {
				r.Unlock()
				continue
			}
			if i > r.commitIndex {
				r.commitIndex = i
			}
			ci := r.commitIndex
			r.Unlock()
			r.applyCommits(ci)
			r.ccv.Broadcast()
		}
	}
}

// makeAppendMsg creates an append message at most 10 entries long.
func (r *raft) makeAppendMsg(m *member) ([]interface{}, int) {
	// Figure out if we know the previous entry.
	prevTerm, prevIndex, ok := r.p.LookupPrevious(m.nextIndex)
	if !ok {
		return nil, 0
	}
	// Collect some log entries to send along.  0 is ok.
	var entries []LogEntry
	for i := 0; i < 10; i++ {
		le := r.p.Lookup(m.nextIndex + Index(i))
		if le == nil {
			break
		}
		entries = append(entries, LogEntry{Cmd: le.Cmd, Term: le.Term, Index: le.Index, Type: le.Type})
	}
	return []interface{}{
		r.p.CurrentTerm(),
		r.me.id,
		prevIndex,
		prevTerm,
		r.commitIndex,
		entries,
	}, len(entries)
}

// updateFollower loops trying to update a follower until the follower is updated or we can't proceed.
// It will always send at least one update so will also act as a heartbeat.
func (r *raft) updateFollower(m *member) {
	// Bring this server up to date.
	r.Lock()
	defer r.Unlock()
	for {
		// If we're not the leader we have no followers.
		if r.role != RoleLeader {
			return
		}

		// Collect some log entries starting at m.nextIndex.
		msg, n := r.makeAppendMsg(m)
		if msg == nil {
			// Try sending a snapshot.
			r.Unlock()
			vlog.Infof("@%s sending snapshot to %s", r.me.id, m.id)
			snapIndex, err := r.sendLatestSnapshot(m)
			r.Lock()
			if err != nil {
				// Try again later.
				vlog.Errorf("@%s sending snapshot to %s: %s", r.me.id, m.id, err)
				return
			}
			m.nextIndex = snapIndex + 1
			vlog.Infof("@%s sent snapshot to %s", r.me.id, m.id)
			// Try sending anything following the snapshot.
			continue
		}

		// Send to the follower. We drop the lock while we do this. That means we may stop being the
		// leader in the middle of the call but that's OK as long as we check when we get it back.
		r.Unlock()
		ctx, cancel := context.WithTimeout(r.ctx, time.Duration(2)*time.Second)
		client := v23.GetClient(ctx)
		err := client.Call(ctx, m.id, "AppendToLog", msg, []interface{}{}, options.Preresolved{})
		cancel()
		r.Lock()
		if r.role != RoleLeader {
			// Not leader any more, doesn't matter how he replied.
			return
		}

		if err != nil {
			if !errors.Is(err, ErrOutOfSequence) {
				// A problem other than missing entries.  Retry later.
				vlog.Errorf("@%s updating %s: %s", r.me.id, m.id, err)
				return
			}
			// At this point we know that the follower is missing entries previous to what
			// we just sent.  If we can backup, do it.  Otherwise try sending a snapshot.
			if m.nextIndex <= 1 {
				return
			}
			prev := r.p.Lookup(m.nextIndex - 1)
			if prev == nil {
				return
			}
			// We can back up.
			m.nextIndex--
			continue
		}

		// The follower appended correctly, update indices and tell the server thread that
		// the commit index may need to change.
		m.nextIndex += Index(n)
		logged := m.nextIndex - 1
		if n > 0 {
			r.setMatchIndex(m, logged)
		}

		// The follower is caught up?
		if m.nextIndex > r.p.LastIndex() {
			return
		}
	}
}

func (r *raft) sendLatestSnapshot(m *member) (Index, error) {
	rd, term, index, err := r.p.OpenLatestSnapshot(r.ctx)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(r.ctx, time.Duration(5*60)*time.Second)
	defer cancel()
	client := raftProtoClient(m.id)
	call, err := client.InstallSnapshot(ctx, r.p.CurrentTerm(), r.me.id, term, index, options.Preresolved{})
	if err != nil {
		return 0, err
	}
	sstream := call.SendStream()
	b := make([]byte, 10240)
	for {
		n, err := rd.Read(b)
		if n == 0 && err == io.EOF {
			break
		}
		if err = sstream.Send(b); err != nil {
			return 0, err
		}
	}
	if err := call.Finish(); err != nil {
		return 0, err
	}
	return index, nil
}

func emptyChan(c chan struct{}) {
	for {
		select {
		case <-c:
		default:
			return
		}
	}
}

// perFollower is a go routine that sequences all messages to a single follower.
//
// This is the only go routine that updates the follower's variables so all changes to
// the member struct are serialized by it.
func (r *raft) perFollower(m *member) {
	m.timer = time.NewTimer(r.heartbeat)
	for {
		select {
		case <-m.timer.C:
			r.updateFollower(m)
			m.timer.Reset(r.heartbeat)
		case <-m.update:
			// Soak up any waiting update requests
			emptyChan(m.update)
			r.updateFollower(m)
			m.timer.Reset(r.heartbeat)
		case <-r.stop:
			close(m.stopped)
			return
		}
	}
}

// kickFollowers causes each perFollower routine to try to update its followers.
func (r *raft) kickFollowers() {
	for _, m := range r.memberMap {
		select {
		case m.update <- struct{}{}:
		default:
		}
	}
}

// setMatchIndex updates the matchIndex for a member.
//
// called with r locked.
func (r *raft) setMatchIndex(m *member, i Index) {
	m.matchIndex = i
	if i <= r.commitIndex {
		return
	}
	// Check if we need to change the commit index.
	select {
	case r.newMatch <- struct{}{}:
	default:
	}
}

func roleToString(r int) string {
	switch r {
	case RoleCandidate:
		return "candidate"
	case RoleLeader:
		return "leader"
	case RoleFollower:
		return "follower"
	}
	return "?"
}

func (r *raft) waitForApply(ctx *context.T, term Term, index Index) (error, error) {
	r.Lock()
	defer r.Unlock()
	for {
		if r.applied.index >= index {
			if term == 0 {
				// Special case: we don't care about Apply() error or committed term, only that we've reached index.
				return nil, nil
			}
			le := r.p.Lookup(index)
			if le == nil || le.Term != term {
				// There was an election and the log entry was lost.
				return nil, ErrorfNotLeader(ctx, "not the leader")
			}
			return le.ApplyError, nil
		}

		// Give up if the caller doesn't want to wait.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Wait for an apply to happen.  r will be unlocked during the wait.
		r.ccv.Wait()
	}
}

// waitForLeadership waits until there is an elected leader.
func (r *raft) waitForLeadership(ctx *context.T) (string, int, Index, bool) {
	r.Lock()
	defer r.Unlock()
	for len(r.leader) == 0 {
		// Give up if the caller doesn't want to wait.
		select {
		case <-ctx.Done():
			return "", 0, 0, true
		default:
		}
		r.lcv.Wait()
	}
	return r.leader, r.role, r.commitIndex, false
}

// Append tells the leader to append to the log.  The first error is the result of the client.Apply.  The second
// is any error from raft.
func (r *raft) Append(ctx *context.T, cmd []byte) (error, error) {
	for {
		leader, role, _, timedOut := r.waitForLeadership(ctx)
		if timedOut {
			return nil, ctx.Err()
		}
		switch role {
		case RoleLeader:
			term, index, err := r.s.Append(ctx, nil, cmd)
			if err == nil {
				// We were the leader and the entry has now been applied.
				return r.waitForApply(ctx, term, index)
			}
			// If the leader can't do it, give up.
			if !errors.Is(err, ErrNotLeader) {
				return nil, err
			}
		case RoleFollower:
			client := v23.GetClient(ctx)
			var index Index
			var term Term
			if len(leader) == 0 {
				break
			}
			err := client.Call(ctx, leader, "Append", []interface{}{cmd}, []interface{}{&term, &index}, options.Preresolved{})
			if err == nil {
				return r.waitForApply(ctx, term, index)
			}
			// If the leader can't do it, give up.
			if !errors.Is(err, ErrNotLeader) {
				return nil, err
			}
		}

		// Give up if the caller doesn't want to wait.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
}

func (r *raft) Leader() (bool, string) {
	r.Lock()
	defer r.Unlock()
	if r.role == RoleLeader {
		return true, r.leader
	}
	return false, r.leader
}

// syncWithLeader synchronizes with the leader.  On return we have applied the commit index
// that existed before the call.
func (r *raft) syncWithLeader(ctx *context.T) error {
	for {
		leader, role, commitIndex, timedOut := r.waitForLeadership(ctx)
		if timedOut {
			return ctx.Err()
		}

		switch role {
		case RoleLeader:
			r.waitForApply(ctx, 0, commitIndex) //nolint:errcheck
			return nil
		case RoleFollower:
			client := v23.GetClient(ctx)
			var index Index
			err := client.Call(ctx, leader, "Committed", []interface{}{}, []interface{}{&index}, options.Preresolved{})
			if err == nil {
				r.waitForApply(ctx, 0, index) //nolint:errcheck
				return nil
			}
			// If the leader can't do it, give up.
			if !errors.Is(err, ErrNotLeader) {
				return err
			}
		}

		// Give up if the caller doesn't want to wait.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

// syncLoop is a go routine that syncs whenever necessary with the leader.
func (r *raft) syncLoop() {
	for {
		// Wait for someone to request syncing.
		r.sync.Lock()
		for r.sync.requested <= r.sync.done {
			select {
			case <-r.stop:
				close(r.sync.stopped)
				r.sync.Unlock()
				return
			default:
			}
			r.sync.requestedcv.Wait()
		}
		requested := r.sync.requested
		r.sync.Unlock()

		// Perform the sync outside the lock.
		if err := r.syncWithLeader(r.ctx); err != nil {
			continue
		}

		// Wake up waiters.
		r.sync.Lock()
		r.sync.done = requested
		r.sync.Unlock()
		r.sync.donecv.Broadcast()
	}
}

// Sync waits for this member to have applied the current commit indexl.
func (r *raft) Sync(ctx *context.T) error {
	r.sync.Lock()
	defer r.sync.Unlock()
	r.sync.requested++
	requested := r.sync.requested
	r.sync.requestedcv.Broadcast()
	// Wait for our sync to complete.
	for requested > r.sync.done {
		select {
		case <-r.stop:
			return ctx.Err()
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		r.sync.donecv.Wait()
	}
	return nil
}
