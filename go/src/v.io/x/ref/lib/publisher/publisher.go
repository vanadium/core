// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package publisher provides a type to publish names to a mounttable.
package publisher

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/verror"
)

// The publisher adds this much slack to each TTL.
const mountTTLSlack = 20 * time.Second

// T manages the publishing of names and servers in the mounttable.
// It spawns an internal goroutine the periodically performs mount and unmount
// rpcs. T is safe to use concurrently.
type T struct {
	ctx    *context.T         // context used to make rpcs
	cancel context.CancelFunc // cancel function for the above ctx
	ns     namespace.T
	period time.Duration
	closed chan struct{} // closed when the Publisher is closed

	mu      sync.Mutex
	changed chan struct{}
	dirty   chan struct{}
	names   map[string]nameAttr               // names that have been added
	servers map[string]bool                   // servers that have been added
	entries map[publishKey]rpc.PublisherEntry // map each (name,server) to its entry
}

type nameAttr struct {
	servesMT bool
	isLeaf   bool
}

type publishKey struct {
	name, server string
}

// New returns a new publisher that updates mounts on ns every period, and when
// changes are made to the state.
func New(ctx *context.T, ns namespace.T, period time.Duration) *T {
	p := &T{
		ns:      ns,
		period:  period,
		closed:  make(chan struct{}),
		changed: make(chan struct{}, 1),
		dirty:   make(chan struct{}),
		names:   make(map[string]nameAttr),
		servers: make(map[string]bool),
		entries: make(map[publishKey]rpc.PublisherEntry),
	}
	timer := time.NewTimer(period) // timer for the next refresh publish call
	// We create a new root context so that unmount RPCs can work even after the ctx
	// passed in is closed.
	p.ctx, p.cancel = context.WithRootCancel(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				timer.Stop()
				p.stop()
				close(p.closed)
				return
			case <-timer.C:
				timer.Reset(period)
				p.publish(true)
			case <-p.changed:
				p.publish(false)
			}
		}
	}()
	return p
}

// AddName adds a new name for all servers to be mounted as.
func (p *T) AddName(name string, mt bool, leaf bool) {
	defer p.mu.Unlock()
	p.mu.Lock()
	if p.names == nil {
		return
	}
	if attr, exists := p.names[name]; exists && (attr.servesMT == mt && attr.isLeaf == leaf) {
		return
	}
	p.names[name] = nameAttr{mt, leaf}
	for server := range p.servers {
		key := publishKey{name, server}
		if pe, ok := p.entries[key]; ok {
			pe.DesiredState = rpc.PublisherMounted
			p.entries[key] = pe
		} else {
			p.entries[key] = rpc.PublisherEntry{Name: name, Server: server, DesiredState: rpc.PublisherMounted}
		}
	}
	p.notifyChanged()
}

// RemoveName removes a name.
func (p *T) RemoveName(name string) {
	defer p.mu.Unlock()
	p.mu.Lock()
	if p.names == nil {
		return
	}
	if _, exists := p.names[name]; !exists {
		return
	}
	delete(p.names, name)
	for server := range p.servers {
		key := publishKey{name, server}
		if pe, ok := p.entries[key]; ok {
			pe.DesiredState = rpc.PublisherUnmounted
			p.entries[key] = pe
		}
	}
	p.notifyChanged()
}

// AddServer adds a new server to be mounted under all names.
func (p *T) AddServer(server string) {
	defer p.mu.Unlock()
	p.mu.Lock()
	if p.names == nil {
		return
	}
	if _, exists := p.servers[server]; exists {
		return
	}
	p.servers[server] = true
	for name := range p.names {
		key := publishKey{name, server}
		if pe, ok := p.entries[key]; ok {
			pe.DesiredState = rpc.PublisherMounted
			p.entries[key] = pe
		} else {
			p.entries[key] = rpc.PublisherEntry{Name: name, Server: server, DesiredState: rpc.PublisherMounted}
		}
	}
	p.notifyChanged()
}

// RemoveServer removes a server from the list of mounts.
func (p *T) RemoveServer(server string) {
	defer p.mu.Unlock()
	p.mu.Lock()
	if p.names == nil {
		return
	}
	if _, exists := p.servers[server]; !exists {
		return
	}
	delete(p.servers, server)
	for name := range p.names {
		key := publishKey{name, server}
		if pe, ok := p.entries[key]; ok {
			pe.DesiredState = rpc.PublisherUnmounted
			p.entries[key] = pe
		}
	}
	p.notifyChanged()
}

// Status returns a snapshot of the publisher's current state.
// The returned channel is closed when the state has become stale and the caller
// should repoll Status.
func (p *T) Status() ([]rpc.PublisherEntry, <-chan struct{}) {
	defer p.mu.Unlock()
	p.mu.Lock()
	st := make([]rpc.PublisherEntry, 0, len(p.entries))
	now := time.Now()
	for _, e := range p.entries {
		mountDelta := now.Sub(e.LastMount)
		switch {
		case e.LastMount.IsZero() && e.LastUnmount.IsZero():
			e.LastState = rpc.PublisherUnmounted
		case e.LastUnmount.After(e.LastMount) && e.LastUnmountErr == nil:
			e.LastState = rpc.PublisherUnmounted
		case mountDelta > p.period+2*mountTTLSlack:
			e.LastState = rpc.PublisherUnmounted
		case mountDelta < p.period:
			e.LastState = rpc.PublisherMounted
		case e.LastUnmount.After(e.LastMount):
			e.LastState = rpc.PublisherUnmounting
		default:
			e.LastState = rpc.PublisherMounting
		}
		st = append(st, e)
	}
	return st, p.dirty
}

// String returns a string representation of the publisher.
func (p *T) String() string {
	defer p.mu.Unlock()
	p.mu.Lock()
	l := make([]string, 0, 2+len(p.entries))
	l = append(l, fmt.Sprintf("Publisher period:%v", p.period))
	l = append(l, "==============================Mounts============================================")
	for key, entry := range p.entries {
		l = append(l, fmt.Sprintf("[%s,%s] mount(%v, %v, %v) unmount(%v, %v) Last: %s, Desired: %s ", key.name, key.server,
			entry.LastMount, entry.LastMountErr, entry.TTL, entry.LastUnmount, entry.LastUnmountErr, entry.LastState, entry.DesiredState))
	}
	return strings.Join(l, "\n")
}

// Closed returns a channel that is closed when the publisher context is cancelled,
// and all unmount operations terminate.
func (p *T) Closed() <-chan struct{} {
	return p.closed
}

// publish makes RPCs to the mounttable to mount and unmount entries.
// If refreshAll is true, then all entries will be refreshed.
// Otherwise fresh changes in entries will be updated (i.e. AddName, RemoveName, etc.)
func (p *T) publish(refreshAll bool) {
	mounts, unmounts := p.entriesToPublish(refreshAll)

	// TODO(suharshs): We could potentially do these mount and unmount rpcs in parallel.
	mountEntries := make([]rpc.PublisherEntry, 0, len(mounts))
	unmountEntries := make([]rpc.PublisherEntry, 0, len(unmounts))
	for _, params := range mounts {
		mountEntries = append(mountEntries, p.mount(params))
	}
	for _, params := range unmounts {
		unmountEntries = append(unmountEntries, p.unmount(params))
	}

	// Update p.entries with the new entries.
	p.updateEntries(mountEntries, unmountEntries)
}

func (p *T) entriesToPublish(refreshAll bool) ([]mountParams, []unmountParams) {
	defer p.mu.Unlock()
	p.mu.Lock()
	var mounts []mountParams
	var unmounts []unmountParams
	for key, entry := range p.entries {
		if entry.DesiredState == rpc.PublisherUnmounted {
			if entry.LastState == rpc.PublisherUnmounted {
				delete(p.entries, key)
			} else if refreshAll || entry.LastUnmount.IsZero() {
				unmounts = append(unmounts, unmountParams{entry: entry, retry: true})
			}
		} else {
			if refreshAll || entry.LastMount.IsZero() {
				mounts = append(mounts, mountParams{entry: entry, attr: p.names[key.name]})
			}
		}
	}
	return mounts, unmounts
}

func (p *T) updateEntries(mountEntries, unmountEntries []rpc.PublisherEntry) {
	defer p.mu.Unlock()
	p.mu.Lock()
	for _, entry := range mountEntries {
		key := publishKey{entry.Name, entry.Server}
		// Ensure that the DesiredState, that may have been changed while the
		// lock was released, is not overwritten by the DesiredState of entry.
		if current, ok := p.entries[key]; ok {
			entry.DesiredState = current.DesiredState
		}
		p.entries[key] = entry
	}
	for _, entry := range unmountEntries {
		key := publishKey{entry.Name, entry.Server}
		// Ensure that we don't delete the entry if the DesiredState was
		// changed while the lock was released.
		if current, ok := p.entries[key]; ok {
			entry.DesiredState = current.DesiredState
		}
		if entry.DesiredState == rpc.PublisherUnmounted && entry.LastUnmountErr == nil {
			delete(p.entries, key)
		} else {
			p.entries[key] = entry
		}
	}
	close(p.dirty)
	p.dirty = make(chan struct{})
}

func (p *T) stop() {
	defer p.mu.Unlock()
	p.mu.Lock()
	p.names = nil
	p.servers = nil
	// We make one final attempt to unmount everything; we ignore failures here,
	// and don't retry, since the mounts will eventually timeout anyways.
	for _, pe := range p.entries {
		p.unmount(unmountParams{entry: pe, retry: false})
	}
	p.cancel()
	close(p.dirty)
}

func (p *T) notifyChanged() {
	// We ensure that callers of this function (i.e AddName, RemoveName, AddServer
	// RemoveServer) do not block. We do this by giving p.changed a buffer of size 1,
	// and adding a default clause below. This allows multiple calls to notifyChanged
	// to complete before the internal goroutine processes the change.
	select {
	case p.changed <- struct{}{}:
	default:
	}
}

type mountParams struct {
	entry rpc.PublisherEntry
	attr  nameAttr
}

// mount makes an mount RPC to the entry described in params. It returns a new
// rpc.PublisherEntry, updated with the results of the RPC.
func (p *T) mount(params mountParams) rpc.PublisherEntry {
	last, entry, attr := params.entry, params.entry, params.attr
	// Always mount with ttl = period + slack.
	// The next call to publish call will occur within the next period.
	ttl := p.period + mountTTLSlack
	entry.LastMount = time.Now()
	// Ensure that LastMount > LastUnmount to make it easier to check for the last
	// tried operation.
	if entry.LastMount.Before(entry.LastUnmount) {
		entry.LastMount = entry.LastUnmount.Add(1)
	}
	entry.LastMountErr = p.ns.Mount(p.ctx, entry.Name, entry.Server, ttl, naming.ServesMountTable(attr.servesMT), naming.IsLeaf(attr.isLeaf))
	entry.TTL = ttl
	// If the mount entry changed, log it.
	if entry.LastMountErr != nil {
		if verror.ErrorID(last.LastMountErr) != verror.ErrorID(entry.LastMountErr) || p.ctx.V(2) {
			p.ctx.Errorf("rpc pub: couldn't mount(%v, %v, %v): %v", entry.Name, entry.Server, ttl, entry.LastMountErr)
		}
	} else {
		entry.LastState = rpc.PublisherMounted
		if last.LastMount.IsZero() || last.LastMountErr != nil || p.ctx.V(2) {
			p.ctx.Infof("rpc pub: mount(%v, %v, %v)", entry.Name, entry.Server, ttl)
		}
	}
	return entry
}

type unmountParams struct {
	entry rpc.PublisherEntry
	retry bool
}

// unmount makes an unmount RPC to the entry described in params. It returns a
// new rpc.PublisherEntry, updated with the results of the RPC.
func (p *T) unmount(params unmountParams) rpc.PublisherEntry {
	entry := params.entry
	var opts []naming.NamespaceOpt
	if !params.retry {
		opts = []naming.NamespaceOpt{options.NoRetry{}}
	}
	entry.LastUnmount = time.Now()
	// Ensure that LastUnmount > LastMount to make it easier to check for the last
	// tried operation.
	if entry.LastUnmount.Before(entry.LastMount) {
		entry.LastUnmount = entry.LastMount.Add(1)
	}
	entry.LastUnmountErr = p.ns.Unmount(p.ctx, entry.Name, entry.Server, opts...)
	if entry.LastUnmountErr != nil {
		p.ctx.Errorf("rpc pub: couldn't unmount(%v, %v): %v", entry.Name, entry.Server, entry.LastUnmountErr)
	} else {
		entry.LastState = rpc.PublisherUnmounted
		p.ctx.VI(1).Infof("rpc pub: unmount(%v, %v)", entry.Name, entry.Server)
	}
	return entry
}
