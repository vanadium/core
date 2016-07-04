// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Writer is a Client that periodically writes key/value pairs to a set of
// collections in a database.  The time between writes can be configured by
// setting WriteInterval.  The key/value pairs can be configured by setting
// KeyValueFunc.
type Writer struct {
	// Amount of time to wait between db writes.  Defaults to 1 second.
	WriteInterval time.Duration

	// Function that generates successive key/value pairs.  If key is "", no
	// row will be written.
	// If KeyValueFunc is nil, keys will be strings containing the current unix
	// timestamp and values will be random hex strings.
	KeyValueFunc func(time.Time) (string, interface{})

	// Function that will run after each write with the collection, key, value,
	// and an error if one was encountered.  Useful for tests.
	OnWrite func(syncbase.Collection, string, interface{}, error)

	ctx *context.T
	// Map of databases and their respective collections.
	dbColsMap map[syncbase.Database][]syncbase.Collection
	err       error
	stopChan  chan struct{}

	// wg waits for all spawned goroutines to stop.
	wg sync.WaitGroup
}

var _ Client = (*Writer)(nil)

func defaultKeyValueFunc(t time.Time) (string, interface{}) {
	return fmt.Sprintf("%d", t.UnixNano()), fmt.Sprintf("%08x", rand.Int31())
}

func (w *Writer) String() string {
	dbNames := []string{}
	for db := range w.dbColsMap {
		dbNames = append(dbNames, db.Id().Name)
	}
	return strings.Join(append([]string{"Writer"}, dbNames...), "-")
}

func (w *Writer) onTick(t time.Time) error {
	key, value := w.KeyValueFunc(t)
	if key == "" {
		return nil
	}
	for _, colSlice := range w.dbColsMap {
		for _, col := range colSlice {
			err := col.Put(w.ctx, key, value)
			if w.OnWrite != nil {
				w.OnWrite(col, key, value, err)
			}
			w.setError(err)
		}
	}
	return nil
}

func (w *Writer) Start(ctx *context.T, sbName string, databases model.DatabaseSet) {
	w.ctx = ctx
	w.err = nil
	w.stopChan = make(chan struct{})

	interval := w.WriteInterval
	if interval == 0 {
		interval = 1 * time.Second
	}

	if w.KeyValueFunc == nil {
		w.KeyValueFunc = defaultKeyValueFunc
	}

	w.wg.Add(1)
	go func() {
		defer func() {
			w.wg.Done()
			w.stopChan = nil
		}()
		var err error
		w.dbColsMap, _, err = CreateDbsAndCollections(ctx, sbName, databases)
		if err != nil {
			w.setError(err)
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case t := <-ticker.C:
				w.onTick(t)
			case <-w.stopChan:
				return
			}
		}
	}()
}

func (w *Writer) Stop() error {
	if w.stopChan != nil {
		close(w.stopChan)
	}
	w.wg.Wait()
	return w.err
}

func (w *Writer) setError(err error) {
	if err != nil && w.err == nil {
		w.err = err
	}
}
