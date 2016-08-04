// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package timekeeper implements simulated time against the
// v.io/x/ref/lib/timekeeper.TimeKeeper interface.
package timekeeper

import (
	"container/heap"
	"sync"
	"time"

	"v.io/x/ref/lib/timekeeper"
)

// ManualTime is a time keeper that allows control over the advancement of time.
type ManualTime interface {
	timekeeper.TimeKeeper
	// AdvanceTime advances the current time by d.
	AdvanceTime(d time.Duration)
	// Requests provides a channel where the requested delays for After and
	// Sleep can be observed.
	Requests() <-chan time.Duration
}

// item is a heap element: every request for After is added to the heap with its
// wake-up time as key (where wake-up time is current time + requested delay
// duration).  As current time advances, items are plucked from the heap and the
// clients are notified.
type item struct {
	t  time.Time        // Wake-up time.
	ch chan<- time.Time // Client notification channel.
}

type timeHeap []*item

func (th timeHeap) Len() int { return len(th) }

func (th timeHeap) Less(i, j int) bool {
	return th[i].t.Before(th[j].t)
}

func (th timeHeap) Swap(i, j int) {
	th[i], th[j] = th[j], th[i]
}

func (th *timeHeap) Push(x interface{}) {
	item := x.(*item)
	*th = append(*th, item)
}

func (th *timeHeap) Pop() interface{} {
	old := *th
	n := len(old)
	item := old[n-1]
	*th = old[0 : n-1]
	return item
}

// manualTime implements TimeKeeper.
type manualTime struct {
	sync.Mutex
	current  time.Time // The current time.
	schedule timeHeap  // The heap of items still to be woken up.
	requests chan time.Duration
}

// After implements TimeKeeper.After.
func (mt *manualTime) After(d time.Duration) <-chan time.Time {
	defer mt.Unlock()
	mt.Lock()
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- mt.current
	} else {
		heap.Push(&mt.schedule, &item{t: mt.current.Add(d), ch: ch})
	}
	mt.requests <- d
	return ch
}

// Sleep implements TimeKeeper.Sleep.
func (mt *manualTime) Sleep(d time.Duration) {
	<-mt.After(d)
}

// AdvanceTime implements ManualTime.AdvanceTime.
func (mt *manualTime) AdvanceTime(d time.Duration) {
	defer mt.Unlock()
	mt.Lock()
	if d > 0 {
		mt.current = mt.current.Add(d)
	}
	for {
		if mt.schedule.Len() == 0 {
			break
		}
		top := mt.schedule[0]
		if top.t.After(mt.current) {
			break
		}
		top.ch <- mt.current
		heap.Pop(&mt.schedule)
	}
}

// Requests implements ManualTime.Requests.
func (mt *manualTime) Requests() <-chan time.Duration { return mt.requests }

// NewManualTime constructs a new instance of ManualTime, with current time set
// at 0.
func NewManualTime() ManualTime {
	mt := &manualTime{
		schedule: make([]*item, 0),
		// 1000 should be plenty to avoid blocking on adding items
		// to the channel, but technically we can still end up blocked.
		requests: make(chan time.Duration, 1000),
	}
	heap.Init(&mt.schedule)
	return mt
}

// Now implements TimeKeeper.Now.
func (mt *manualTime) Now() time.Time {
	defer mt.Unlock()
	mt.Lock()
	return mt.current
}
