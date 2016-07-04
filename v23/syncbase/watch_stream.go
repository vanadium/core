// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"fmt"
	"strings"
	"sync"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
)

type watchStream struct {
	// cancel cancels the RPC stream.
	cancel context.CancelFunc

	mu sync.Mutex
	// call is the RPC stream object.
	call watch.GlobWatcherWatchGlobClientCall
	// curr is the currently staged element, or nil if nothing is staged.
	curr *WatchChange
	// err is the first error encountered during streaming. It may also be
	// populated by a call to Cancel.
	err error
	// finished records whether we have called call.Finish().
	finished bool
}

var _ WatchStream = (*watchStream)(nil)

func newWatchStream(cancel context.CancelFunc, call watch.GlobWatcherWatchGlobClientCall) *watchStream {
	return &watchStream{
		cancel: cancel,
		call:   call,
	}
}

// Advance implements Stream interface.
func (s *watchStream) Advance() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return false
	}
	for {
		// Advance never blocks if the context has been cancelled.
		if !s.call.RecvStream().Advance() {
			s.err = s.call.Finish()
			s.cancel()
			s.finished = true
			return false
		}
		// Skip initial-state-skipped changes.
		if s.call.RecvStream().Value().State == watch.InitialStateSkipped {
			continue
		}
		s.curr = ToWatchChange(s.call.RecvStream().Value())
		return true
	}
}

// Change implements WatchStream interface.
func (s *watchStream) Change() WatchChange {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.curr == nil {
		panic("nothing staged")
	}
	return *s.curr
}

// Err implements Stream interface.
func (s *watchStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		return nil
	}
	return verror.Convert(verror.IDAction{}, nil, s.err)
}

// Cancel implements Stream interface.
func (s *watchStream) Cancel() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.finished {
		s.err = s.call.Finish()
		s.finished = true
	}
}

// ToWatchChange converts a generic Change struct as defined in
// v.io/v23/services/watch to a Syncbase-specific WatchChange struct as defined
// in v.io/v23/syncbase.
func ToWatchChange(c watch.Change) *WatchChange {
	// TODO(ivanpi): Store the errors in err instead of panicking.
	// Parse the store change.
	var storeChange wire.StoreChange
	if err := c.Value.ToValue(&storeChange); err != nil {
		panic(fmt.Errorf("ToValue StoreChange failed: %v, RawBytes: %#v", err, c.Value))
	}
	res := &WatchChange{
		value:        storeChange.Value,
		FromSync:     storeChange.FromSync,
		Continued:    c.Continued,
		ResumeMarker: c.ResumeMarker,
	}
	// Convert the state.
	switch c.State {
	case watch.Exists:
		res.ChangeType = PutChange
	case watch.DoesNotExist:
		res.ChangeType = DeleteChange
	default:
		panic(fmt.Sprintf("unsupported watch change state: %v", c.State))
	}
	var err error
	// Parse the name and determine the entity type.
	if c.Name == "" {
		res.EntityType = EntityRoot
		// No other fields need to be set for EntityRoot.
	} else if strings.ContainsRune(c.Name, '/') {
		res.EntityType = EntityRow
		// Parse the collection id and row key.
		if res.Collection, res.Row, err = util.ParseCollectionRowPair(nil, c.Name); err != nil {
			panic(err)
		}
		if res.Row == "" {
			panic("empty row name")
		}
	} else {
		res.EntityType = EntityCollection
		// Parse the collection id.
		if res.Collection, err = util.DecodeId(c.Name); err != nil {
			panic(err)
		}
		if res.ChangeType == PutChange {
			// Verify that the collection info is decodable.
			if err := storeChange.Value.ToValue(&wire.StoreChangeCollectionInfo{}); err != nil {
				panic(fmt.Errorf("ToValue StoreChangeCollectionInfo failed: %v, RawBytes: %#v", err, storeChange.Value))
			}
		}
	}
	return res
}
