// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raft

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/x/lib/vlog"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type client struct {
	sync.RWMutex
	cmds    [][]byte // applied commands
	id      string
	applied Index // highest index applied
}

func (c *client) Apply(cmd []byte, index Index) error {
	vlog.VI(2).Infof("Applying %d %s", index, cmd)
	c.Lock()
	c.cmds = append(c.cmds, cmd)
	c.applied = index
	c.Unlock()
	return nil
}

func (c *client) Applied() Index {
	c.RLock()
	defer c.RUnlock()
	return c.applied
}

func (c *client) TotalApplied() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.cmds)
}

func (c *client) SaveToSnapshot(ctx *context.T, wr io.Writer, response chan<- error) error {
	close(response)
	c.RLock()
	defer c.RUnlock()
	return json.NewEncoder(wr).Encode(c.cmds)
}

func (c *client) RestoreFromSnapshot(ctx *context.T, index Index, rd io.Reader) error {
	c.Lock()
	defer c.Unlock()
	c.applied = index
	return json.NewDecoder(rd).Decode(&c.cmds)
}

func (c *client) LeaderChange(me, leader string) {
	if me == leader {
		vlog.Infof("%s now leader", leader)
	} else {
		vlog.Infof("%s recognizes %s as leader", me, leader)
	}
}

func (c *client) Compare(t *testing.T, nc *client) {
	c.RLock()
	defer c.RUnlock()
	nc.RLock()
	defer nc.RUnlock()
	if !reflect.DeepEqual(c.cmds, nc.cmds) {
		t.Fatalf("%v != %v", c.cmds, nc.cmds)
	}
}

// buildRafts creates a set of raft members and starts up the services.
func buildRafts(t *testing.T, ctx *context.T, n int, config *RaftConfig) ([]*raft, []*client) {
	if config == nil {
		config = new(RaftConfig)
	}
	config.Heartbeat = time.Second
	if len(config.HostPort) == 0 {
		config.HostPort = "127.0.0.1:0"
	}
	// Start each server with its own log directory.
	var rs []*raft
	var cs []*client
	for i := 0; i < n; i++ {
		if n > 1 || len(config.LogDir) == 0 {
			config.LogDir = tempDir(t)
		}
		c := new(client)
		r, err := newRaft(ctx, config, c)
		if err != nil {
			t.Fatalf("NewRaft: %s", err)
		}
		c.id = r.ID()
		rs = append(rs, r)
		cs = append(cs, c)
		vlog.Infof("id is %s", r.ID())
	}
	// Tell each server about the complete set.
	for i := range rs {
		for j := range rs {
			rs[i].AddMember(ctx, rs[j].ID()) //nolint:errcheck
		}
	}
	// Start the servers up.
	for i := range rs {
		rs[i].Start()
	}
	return rs, cs
}

// restart a member from scratch, keeping its address and log name.
func restart(t *testing.T, ctx *context.T, rs []*raft, cs []*client, r *raft) {
	config := RaftConfig{HostPort: r.me.id[1:], LogDir: r.logDir}
	c := new(client)
	rn, err := newRaft(ctx, &config, c)
	if err != nil {
		t.Fatalf("NewRaft: %s", err)
	}
	for i := range rs {
		if rs[i] == r {
			rs[i] = rn
			cs[i] = c
			c.id = rn.ID()
			break
		}
	}
	for j := range rs {
		rn.AddMember(ctx, rs[j].ID()) //nolint:errcheck
	}
	rn.Start()
}

// cleanUp all the rafts.
func cleanUp(rs []*raft) {
	for i := range rs {
		rs[i].Stop()
		os.RemoveAll(rs[i].logDir)
	}
}

func TestClientSnapshot(t *testing.T) {
	vlog.Infof("TestClientSnapshot")
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Make sure the test client works as expected.
	c := new(client)
	for i, cmd := range []string{"the", "rain", "in", "spain", "falls", "mainly", "on", "the", "plain"} {
		c.Apply([]byte(cmd), Index(i)) //nolint:errcheck
	}
	fp, err := ioutil.TempFile(".", "TestClientSnapshot")
	if err != nil {
		t.Fatalf("can't create snapshot: %s", err)
	}
	done := make(chan error)
	if err := c.SaveToSnapshot(ctx, fp, done); err != nil {
		t.Fatalf("can't write snapshot: %s", err)
	}
	<-done
	fp.Sync()     //nolint:errcheck
	fp.Seek(0, 0) //nolint:errcheck
	nc := new(client)
	if err != nil {
		t.Fatalf("can't open snapshot: %s", err)
	}
	if err := nc.RestoreFromSnapshot(ctx, 0, fp); err != nil {
		t.Fatalf("can't read snapshot: %s", err)
	}
	fp.Close()
	os.Remove(fp.Name())
	c.Compare(t, nc)
	vlog.Infof("TestCreation passed")
}
