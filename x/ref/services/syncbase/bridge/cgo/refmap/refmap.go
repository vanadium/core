// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package refmap

import (
	"sync"
)

func NewRefMap() *refMap {
	return &refMap{
		refs: make(map[uint64]interface{}),
	}
}

type refMap struct {
	refs   map[uint64]interface{}
	lastId uint64
	lock   sync.Mutex
}

func (r *refMap) Add(val interface{}) uint64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	id := r.lastId
	r.lastId++
	r.refs[id] = val
	return id
}

func (r *refMap) Get(id uint64) interface{} {
	r.lock.Lock()
	defer r.lock.Unlock()
	if val, ok := r.refs[id]; ok {
		return val
	}
	return nil
}

func (r *refMap) Remove(id uint64) interface{} {
	r.lock.Lock()
	defer r.lock.Unlock()
	if val, ok := r.refs[id]; ok {
		delete(r.refs, id)
		return val
	}
	return nil
}
