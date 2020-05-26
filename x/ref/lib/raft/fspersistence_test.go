// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raft

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"v.io/x/lib/vlog"
	"v.io/x/ref/test"
)

func tempDir(t *testing.T) string {
	// Get a temporary file name that is unlikely to clash with anyone else's.
	dir, err := ioutil.TempDir("", "raftstate")
	if err != nil {
		t.Fatalf("TempDir: %s", err)
	}
	os.Remove(dir)
	vlog.Infof("log is %s", dir)
	return dir
}

func compareLogs(t *testing.T, p persistent, expected []LogEntry, tag string) {
	found := 0
outer:
	for i := Index(0); i < 1000; i++ {
		le := p.Lookup(i)
		if le == nil {
			continue
		}
		for _, v := range expected {
			if v.Index != le.Index {
				continue
			}
			if v.Term != le.Term || string(v.Cmd) != string(le.Cmd) {
				t.Fatalf("%s: expected %v, got %v", tag, v, *le)
			}
			found++
			continue outer
		}
		t.Fatalf("unexpected: %v", *le)
	}
	if found != len(expected) {
		t.Fatalf("%s: not all entries found, got %d out of %v", tag, found, expected)
	}
}

// TestCreation verifies that a log is created when we start a fresh instance and is still there after we stop it.
func TestCreation(t *testing.T) {
	vlog.Infof("TestCreation")
	ctx, shutdown := test.V23Init()
	defer shutdown()
	td := tempDir(t)
	defer os.RemoveAll(td)

	// Launch a raft.  This should create the log file and append at least one control entry to it.
	config := RaftConfig{HostPort: "127.0.0.1:0", LogDir: td}
	r, err := newRaft(ctx, &config, new(client))
	if err != nil {
		t.Fatalf("newRaft: %s", err)
	}
	vlog.Infof("id is %s", r.ID())

	// Make sure the file is there.
	info, err := os.Stat(path.Join(td, "00000000000000000000.00000000000000000000.log"))
	if err != nil {
		t.Fatalf("File didn't last: %s", err)
	}
	if info.Size() == 0 {
		t.Fatalf("File too short: %d", info.Size())
	}
	r.Stop()

	// Make sure the file is still there after shutdown.
	info, err = os.Stat(path.Join(td, "00000000000000000000.00000000000000000000.log"))
	if err != nil {
		t.Fatalf("File didn't last: %s", err)
	}
	if info.Size() == 0 {
		t.Fatalf("File too short: %d", info.Size())
	}
	vlog.Infof("TestCreation passed")
}

// TestPersistence verifies that we store enough state to return to the previous log state
// after a orderly stop and restart.
//
// We also test that a truncated log remains truncated after an orderly stop and restart.
func TestPersistence(t *testing.T) {
	vlog.Infof("TestPersistence")
	ctx, shutdown := test.V23Init()
	defer shutdown()
	td := tempDir(t)
	defer os.RemoveAll(td)

	// Launch a raft.  This should create the file and populate the header with the
	// nil values.  The file should persist after the raft is closed.
	config := RaftConfig{HostPort: "127.0.0.1:0", LogDir: td}
	r, err := newRaft(ctx, &config, new(client))
	if err != nil {
		t.Fatalf("newRaft: %s", err)
	}
	vlog.Infof("id is %s", r.ID())

	// Set the persistent state.
	if err := r.p.SetCurrentTerm(2); err != nil {
		t.Fatalf("SetCurrentTerm: %s", err)
	}
	if err := r.p.SetVotedFor("whocares"); err != nil {
		t.Fatalf("SetVotedFor: %s", err)
	}
	cmds1 := []LogEntry{
		{Term: 2, Index: 1, Cmd: []byte("cmd1")},
		{Term: 2, Index: 2, Cmd: []byte("cmd2")},
		{Term: 2, Index: 3, Cmd: []byte("cmd3")},
	}
	if err := r.p.AppendToLog(ctx, 0, 0, cmds1); err != nil {
		t.Fatalf("AppendToLog: %s", err)
	}
	cmds2 := []LogEntry{
		{Term: 3, Index: 4, Cmd: []byte("cmd4")},
		{Term: 3, Index: 5, Cmd: []byte("cmd5")},
		{Term: 3, Index: 6, Cmd: []byte("cmd6")},
	}
	if err := r.p.AppendToLog(ctx, 2, 3, cmds2); err != nil {
		t.Fatalf("AppendToLog: %s", err)
	}
	info, err := os.Stat(path.Join(td, "00000000000000000000.00000000000000000000.log"))
	if err != nil {
		t.Fatalf("File didn't last: %s", err)
	}
	vlog.Infof("log size %d", info.Size())
	vlog.Infof("stopping %s", r.ID())
	r.Stop()

	// Reopen the persistent store and make sure it matches.
	r, err = newRaft(ctx, &config, new(client))
	if err != nil {
		t.Fatalf("newRaft: %s", err)
	}
	vlog.Infof("id is %s", r.ID())
	if r.p.CurrentTerm() != 2 {
		t.Fatalf("CurrentTerm: expected %d got %d", 2, r.p.CurrentTerm())
	}
	if r.p.VotedFor() != "whocares" {
		t.Fatalf("CurrentTerm: expected %s got %s", "whocares", r.p.VotedFor())
	}
	test := []LogEntry{
		{Term: 2, Index: 1, Cmd: []byte("cmd1")},
		{Term: 2, Index: 2, Cmd: []byte("cmd2")},
		{Term: 2, Index: 3, Cmd: []byte("cmd3")},
		{Term: 3, Index: 4, Cmd: []byte("cmd4")},
		{Term: 3, Index: 5, Cmd: []byte("cmd5")},
		{Term: 3, Index: 6, Cmd: []byte("cmd6")},
	}
	compareLogs(t, r.p, test, "after reopen")

	// Truncate the log by rewriting an index.
	if err := r.p.AppendToLog(ctx, 2, 3, []LogEntry{{Term: 4, Index: 4, Cmd: []byte("cmd7")}}); err != nil {
		t.Fatalf("AppendToLog: %s", err)
	}
	test2 := []LogEntry{
		{Term: 2, Index: 1, Cmd: []byte("cmd1")},
		{Term: 2, Index: 2, Cmd: []byte("cmd2")},
		{Term: 2, Index: 3, Cmd: []byte("cmd3")},
		{Term: 4, Index: 4, Cmd: []byte("cmd7")},
	}
	compareLogs(t, r.p, test2, "after truncate")
	vlog.Infof("stopping %s", r.ID())
	r.Stop()

	// Make sure the log is still truncated when we reread it.
	r, err = newRaft(ctx, &config, new(client))
	if err != nil {
		t.Fatalf("newRaft: %s", err)
	}
	vlog.Infof("id is %s", r.ID())
	compareLogs(t, r.p, test2, "after truncate and close")
	vlog.Infof("stopping %s", r.ID())
	r.Stop()
	vlog.Infof("TestPersistence passed")
}

func getFileNames(t *testing.T, dir string) []string {
	d, err := os.Open(dir)
	if err != nil {
		t.Fatalf("opening %s: %s", dir, err)
	}
	defer d.Close()

	// Find all snapshot and log files.
	files, err := d.Readdirnames(0)
	if err != nil {
		t.Fatalf("reading %s: %s", dir, err)
	}

	return files
}

// waitForLeadership waits for r to be elected leader.
func waitForLeadership(r *raft, timeout time.Duration) bool {
	start := time.Now()
	for {
		_, role, _ := r.Status()
		if role == RoleLeader {
			return true
		}
		if time.Since(start) > timeout {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSnapshot sets the limit for taking a snapshot down to 1 record and makes sure that snapshots are taken.
func TestSnapshot(t *testing.T) {
	vlog.Infof("TestSnapshot")
	ctx, shutdown := test.V23Init()
	defer shutdown()
	td := tempDir(t)
	defer os.RemoveAll(td)

	config := RaftConfig{HostPort: "127.0.0.1:0", LogDir: td, SnapshotThreshold: 1}
	rs, _ := buildRafts(t, ctx, 1, &config)
	defer cleanUp(rs)
	r := rs[0]
	c1 := r.client
	vlog.Infof("id is %s", r.ID())

	// This should cause a snapshot.
	if apperr, err := r.Append(ctx, []byte("string1")); err != nil || apperr != nil {
		t.Fatalf("AppendToLog: %s/%s", apperr, err)
	}
	if apperr, err := r.Append(ctx, []byte("string2")); err != nil || apperr != nil {
		t.Fatalf("AppendToLog: %s/%s", apperr, err)
	}
	if apperr, err := r.Append(ctx, []byte("string3")); err != nil || apperr != nil {
		t.Fatalf("AppendToLog: %s/%s", apperr, err)
	}

	// Wait for the snapshot to appear.  It is a background task so just loop.
	deadline := time.Now().Add(time.Minute)
outer:
	for {
		time.Sleep(100 * time.Millisecond)
		files := getFileNames(t, td)
		for _, s := range files {
			if strings.HasSuffix(s, ".snap") {
				log := strings.TrimSuffix(s, ".snap") + ".log"
				for _, l := range files {
					if l == log {
						break outer
					}
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for snap")
		}
	}
	r.Stop()

	// Restart.  We should read and restart from the snapshot.
	rs, _ = buildRafts(t, ctx, 1, &config)
	defer cleanUp(rs)
	r = rs[0]
	c2 := r.client
	vlog.Infof("new id is %s", r.ID())

	// Wait for leadership.
	if !waitForLeadership(r, time.Minute) {
		t.Fatalf("didn't become leader")
	}

	// Wait to commit.
	time.Sleep(time.Second)
	c1.(*client).Compare(t, c2.(*client))

	vlog.Infof("TestSnapshot passed")
}

// TestRemoteSnapshot makes sure a member can be recovered from a snapshot taken elsewhere.
func TestRemoteSnapshot(t *testing.T) {
	vlog.Infof("TestRemoteSnapshot")
	ctx, shutdown := test.V23Init()
	defer shutdown()
	config := RaftConfig{HostPort: "127.0.0.1:0", SnapshotThreshold: 30}
	rs, cs := buildRafts(t, ctx, 3, &config)
	defer cleanUp(rs)

	// This should cause a few snapshots.
	for i := 0; i < 100; i++ {
		if apperr, err := rs[1].Append(ctx, []byte(fmt.Sprintf("string%d", i))); err != nil || apperr != nil {
			t.Fatalf("Append: %s/%s", apperr, err)
		}
	}
	vlog.Infof("Appends done")

	// Wait for them all to get to the same point.
	if !waitForLogAgreement(rs, time.Minute) {
		t.Fatalf("no log agreement")
	}
	t.Log("Logs agree")
	if !waitForAppliedAgreement(rs, cs, time.Minute) {
		t.Fatalf("no applied agreement")
	}
	vlog.Infof("Applies agree")

	// Stop a server and remove all its log and snapshot info, i.e., act like its starting
	// from scratch.
	rs[0].Stop()
	os.RemoveAll(rs[0].logDir)
	restart(t, ctx, rs, cs, rs[0])

	// Wait for them all to get to the same point.
	if !waitForLogAgreement(rs, time.Minute) {
		t.Fatalf("no log agreement")
	}
	if !waitForAppliedAgreement(rs, cs, time.Minute) {
		t.Fatalf("no applied agreement")
	}

	vlog.Infof("TestRemoteSnapshot passed")
}
