// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mem provides a simple, in-memory implementation of
// server.Store. Since it's a prototype implementation, it doesn't
// bother with entry-level locking.
package mem

import (
	"strconv"
	"sync"

	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/x/ref/services/groups/internal/store"
)

type entry struct {
	Value   interface{}
	Version uint64
}

type memstore struct {
	mu   sync.Mutex
	err  error
	data map[string]*entry
}

var _ store.Store = (*memstore)(nil)

func New() store.Store {
	return &memstore{data: map[string]*entry{}}
}

func (st *memstore) Get(k string, v interface{}) (version string, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return "", convertError(st.err)
	}
	e, ok := st.data[k]
	if !ok {
		return "", store.ErrUnknownKey.Errorf(nil, "unknown key %s", k)
	}
	if err := vdl.Convert(v, e.Value); err != nil {
		return "", convertError(err)
	}
	return strconv.FormatUint(e.Version, 10), nil
}

func (st *memstore) Insert(k string, v interface{}) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return convertError(st.err)
	}
	if _, ok := st.data[k]; ok {
		return store.ErrKeyExists.Errorf(nil, "key exists %s", k)
	}
	st.data[k] = &entry{Value: v}
	return nil
}

func (st *memstore) Update(k string, v interface{}, version string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return convertError(st.err)
	}
	e, ok := st.data[k]
	if !ok {
		return store.ErrUnknownKey.Errorf(nil, "unknown key %s", k)
	}
	if err := e.checkVersion(version); err != nil {
		return err
	}
	st.data[k] = &entry{Value: v, Version: e.Version + 1}
	return nil
}

func (st *memstore) Delete(k string, version string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return convertError(st.err)
	}
	e, ok := st.data[k]
	if !ok {
		return store.ErrUnknownKey.Errorf(nil, "unknown key %s", k)
	}
	if err := e.checkVersion(version); err != nil {
		return err
	}
	delete(st.data, k)
	return nil
}

func (st *memstore) Close() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return convertError(st.err)
	}
	st.err = verror.ErrCanceled.Errorf(nil, "canceled: closed store")
	return nil
}

// Internal helpers

func (e *entry) checkVersion(version string) error {
	newVersion := strconv.FormatUint(e.Version, 10)
	if version != newVersion {
		return verror.ErrBadVersion.Errorf(nil, "version is out of date")
	}
	return nil
}

func convertError(err error) error {
	return verror.ErrUnknown.Errorf(nil, "store.mem: %v", err)
}
