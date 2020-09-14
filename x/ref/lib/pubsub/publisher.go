// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pubsub

import (
	"fmt"
	"strings"
	"sync"
)

// A Publisher provides a mechanism for communicating Settings from a set
// of producers to multiple consumers. Each such producer and associated
// consumers are called a Stream. Operations are provided for creating
// streams (CreateStream) and adding new consumers (ForkStream). Communication
// is implemented using channels, with the producer and consumers providing
// channels to send and receive Settings over. A Stream remembers the last value
// of all Settings that were sent over it; these can be retrieved via ForkStream
// or the Latest method.
//
// The Publisher may be shut down by calling its Shutdown method and
// the producers will be notified via the channel returned by CreateStream,
// at which point they should close the channel they use for publishing Settings.
// If producers fail to close this channel then the Publisher will leak
// goroutines (one per stream) when it is shutdown.
type Publisher struct {
	mu       sync.RWMutex
	stop     chan struct{}
	shutdown bool
	streams  map[string]*fork
}

// Stream is returned by Latest and includes the name and description
// for the stream and the most recent values of the Setting that flowed
// through it.
type Stream struct {
	Name, Description string
	// Latest is a map of Setting names to the Setting itself.
	Latest map[string]Setting
}

// NewPublisher creates a Publisher.
func NewPublisher() *Publisher {
	return &Publisher{
		streams: make(map[string]*fork),
		stop:    make(chan struct{}),
	}
}

type fork struct {
	sync.RWMutex
	desc string
	vals map[string]Setting
	in   <-chan Setting
	outs []chan<- Setting
}

// CreateStream creates a Stream with the provided name and description
// (note, Settings have their own names and description, these are for the
// stream). In general, no buffering is required for this channel since
// the Publisher itself will read from it, however, if the consumers are slow
// then the publisher may be slow in draining the channel. The publisher
// should provide additional buffering if this is a concern.
// Consequently this mechanism should be used for rarely changing Settings,
// such as network address changes forced by DHCP and hence no buffering
// will be required. The channel returned by CreateStream is closed when the
// publisher is shut down and hence the caller should wait for this to occur
// and then close the channel it has passed to CreateStream.
func (p *Publisher) CreateStream(name, description string, ch <-chan Setting) (<-chan struct{}, error) {
	if ch == nil {
		return nil, fmt.Errorf("must provide a non-nil channel")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.streams[name] != nil {
		return nil, fmt.Errorf("stream %v already exists", name)
	}
	f := &fork{desc: description, in: ch, vals: make(map[string]Setting)}
	p.streams[name] = f
	go f.flow(p.stop)
	return p.stop, nil
}

// String returns a string representation of the publisher, including
// the names and descriptions of all the streams it currently supports.
func (p *Publisher) String() string {
	r := ""
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.shutdown {
		return "shutdown"
	}
	for k, s := range p.streams {
		r += fmt.Sprintf("(%s: %s) ", k, s.desc)
	}
	return strings.TrimRight(r, " ")
}

// Latest returns information on the requested stream, including the
// last instance of all Settings, if any, that flowed over it.
func (p *Publisher) Latest(name string) *Stream {
	p.mu.Lock()
	defer p.mu.Unlock()
	f := p.streams[name]
	if f == nil {
		return nil
	}
	var r map[string]Setting
	f.RLock()
	defer f.RUnlock()
	for k, v := range f.vals {
		r[k] = v
	}
	return &Stream{Name: name, Description: f.desc, Latest: r}
}

// ForkStream 'forks' the named stream to add a new consumer. The channel
// provided is to be used to read Settings sent down the stream. This
// channel will be closed by the Publisher when it is asked to shut down.
// The reader on this channel must be able to keep up with the flow of Settings
// through the Stream in order to avoid blocking all other readers and hence
// should set an appropriate amount of buffering for the channel it passes in.
// ForkStream returns the most recent values of all Settings previously
// sent over the stream, thus allowing its caller to synchronise with the
// stream.
func (p *Publisher) ForkStream(name string, ch chan<- Setting) (*Stream, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.shutdown {
		return nil, fmt.Errorf("stream %v has been shut down", name)
	}
	f := p.streams[name]
	if f == nil {
		return nil, fmt.Errorf("stream %v doesn't exist", name)
	}
	f.Lock()
	defer f.Unlock()
	r := make(map[string]Setting)
	for k, v := range f.vals {
		r[k] = v
	}
	f.outs = append(f.outs, ch)
	return &Stream{Name: name, Description: f.desc, Latest: r}, nil
}

// CloseFork removes the specified channel from the named stream.
// The caller must drain the channel before closing it.
// TODO(cnicolaou): add tests for this.
func (p *Publisher) CloseFork(name string, ch chan<- Setting) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.shutdown {
		return fmt.Errorf("stream %v has been shut down", name)
	}
	f := p.streams[name]
	if f == nil {
		return fmt.Errorf("stream %v doesn't exist", name)
	}
	f.Lock()
	defer f.Unlock()
	for i, v := range f.outs {
		if v == ch {
			f.outs = append(f.outs[0:i], f.outs[i+1:]...)
			break
		}
	}
	return nil
}

// Shutdown initiates the process of stopping the operation of the Publisher.
// All of the channels passed to CreateStream must be closed by their owner
// to ensure that all goroutines are garbage collected.
func (p *Publisher) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.shutdown {
		return
	}
	p.shutdown = true
	close(p.stop)
}

func (f *fork) closeChans() {
	f.Lock()
	for _, o := range f.outs {
		close(o)
	}
	f.outs = nil
	f.Unlock()
}

func (f *fork) flow(stop chan struct{}) {
	closed := false
	for {
		select {
		case <-stop:
			if !closed {
				f.closeChans()
				closed = true
			}
		case val, ok := <-f.in:
			if !ok {
				f.closeChans()
				return
			}
			f.Lock()
			f.vals[val.Name()] = val
			cpy := make([]chan<- Setting, len(f.outs))
			copy(cpy, f.outs)
			f.Unlock()
			for _, o := range cpy {
				// We may well block here.
				o <- val
			}
		}
	}
}
