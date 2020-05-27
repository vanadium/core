// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"sync"
)

// watchers provides synchronization and notifications for a shared resource.
type watchers struct {
	mu      sync.RWMutex
	nextID  int
	clients map[int][]chan struct{}
}

func (p *watchers) lock() {
	p.mu.Lock()
}

func (p *watchers) unlock(id int) {
	for i, c := range p.clients {
		if i != id {
			for _, ch := range c {
				// Non-blocking send. If the channel is full we don't need
				// to send a duplicate flush.
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}
	p.mu.Unlock()
}

func (p *watchers) newID() int {
	p.mu.Lock()
	id := p.nextID
	p.nextID++
	p.mu.Unlock()
	return id
}

func (p *watchers) register(id int) chan struct{} {
	ch := make(chan struct{}, 1)
	p.mu.Lock()
	p.clients[id] = append(p.clients[id], ch)
	p.mu.Unlock()
	return ch
}

func (p *watchers) unregister(id int, ch chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	clients := p.clients[id]
	max := len(clients) - 1
	for i, v := range clients {
		if v == ch {
			clients[i], clients[max], p.clients[id] = clients[max], nil, clients[:max]
			close(ch)
			return
		}
	}
}

func newWatchers() *watchers {
	return &watchers{clients: make(map[int][]chan struct{})}
}
