// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"strings"
	"sync"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
)

// Watcher is a client that watches a range of keys in a set of database collections.
type Watcher struct {
	// Prefix to watch.  Defaults to empty string.
	// TODO(nlacasse): Allow different prefixes per collection?
	Prefix string

	// ResumeMarker to begin watching from.
	// TODO(nlacasse): Allow different ResumeMarkers per collection?
	ResumeMarker watch.ResumeMarker

	// OnChange runs for each WatchChange.  Must be goroutine-safe.  By default
	// this will be a no-op.
	OnChange func(syncbase.WatchChange)

	ctx *context.T
	// Map of databases and their respective collections.
	dbColMap map[syncbase.Database][]syncbase.Collection
	stopChan chan struct{}

	err   error
	errMu sync.Mutex

	// wg waits until all spawned goroutines stop.
	wg sync.WaitGroup
}

var _ Client = (*Watcher)(nil)

func (w *Watcher) String() string {
	dbNames := []string{}
	for db := range w.dbColMap {
		dbNames = append(dbNames, db.Id().Name)
	}
	return strings.Join(append([]string{"Watcher"}, dbNames...), "-")
}
func (w *Watcher) Start(ctx *context.T, sbName string, dbModels model.DatabaseSet) {
	w.ctx = ctx
	w.err = nil
	w.stopChan = make(chan struct{})

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		var err error
		w.dbColMap, _, err = CreateDbsAndCollections(ctx, sbName, dbModels)
		if err != nil {
			w.setError(err)
			return
		}

		for db, colSlice := range w.dbColMap {
			for _, col := range colSlice {
				// Create a watch stream for the collection.
				// TODO(ivanpi): Simplify now that Watch can span collections.
				stream := db.Watch(ctx, w.ResumeMarker, []wire.CollectionRowPattern{
					util.RowPrefixPattern(col.Id(), w.Prefix),
				})
				defer stream.Cancel()

				// Spawn a goroutine to repeatedly call stream.Advance() and
				// process any changes.
				w.wg.Add(1)
				go func() {
					defer w.wg.Done()
					for {
						advance := stream.Advance()
						if !advance {
							if err := stream.Err(); err != nil && verror.ErrorID(err) != verror.ErrCanceled.ID {
								w.setError(err)
							}
							return
						}
						change := stream.Change()
						if w.OnChange != nil {
							w.OnChange(change)
						}
					}
				}()
			}
		}

		// Wait for stopChan to close.
		<-w.stopChan
	}()
}

func (w *Watcher) Stop() error {
	close(w.stopChan)
	w.wg.Wait()
	return w.err
}

func (w *Watcher) setError(err error) {
	w.errMu.Lock()
	defer w.errMu.Unlock()
	if err != nil && w.err == nil {
		w.err = err
	}
}
