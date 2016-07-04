// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"strings"
	"sync"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
)

// Noop is a client that creates any databases, collections, and syncgroups in
// the model, and then does nothing.
type Noop struct {
	// Map of databases and their respective collections.
	dbColMap map[syncbase.Database][]syncbase.Collection

	err error
	wg  sync.WaitGroup
}

var _ Client = (*Noop)(nil)

func (noop *Noop) String() string {
	dbNames := []string{}
	for db := range noop.dbColMap {
		dbNames = append(dbNames, db.Id().Name)
	}
	return strings.Join(append([]string{"Noop"}, dbNames...), "-")
}

func (noop *Noop) Start(ctx *context.T, sbName string, dbModels model.DatabaseSet) {
	noop.wg.Add(1)
	go func() {
		defer noop.wg.Done()
		noop.dbColMap, _, noop.err = CreateDbsAndCollections(ctx, sbName, dbModels)
	}()
}

func (noop *Noop) Stop() error {
	noop.wg.Wait()
	return noop.err
}
