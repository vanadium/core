// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"v.io/v23/context"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/vom"
)

func newRow(parentFullName, key string, bh wire.BatchHandle) Row {
	// Note, we immediately decode row keys on the server side. See comment in
	// server/dispatcher.go for explanation.
	fullName := naming.Join(parentFullName, util.Encode(key))
	return &row{
		c:        wire.RowClient(fullName),
		fullName: fullName,
		key:      key,
		bh:       bh,
	}
}

type row struct {
	c        wire.RowClientMethods
	fullName string
	key      string
	bh       wire.BatchHandle
}

var _ Row = (*row)(nil)

// Key implements Row.Key.
func (r *row) Key() string {
	return r.key
}

// FullName implements Row.FullName.
func (r *row) FullName() string {
	return r.fullName
}

// Exists implements Row.Exists.
func (r *row) Exists(ctx *context.T) (bool, error) {
	return r.c.Exists(ctx, r.bh)
}

// Get implements Row.Get.
func (r *row) Get(ctx *context.T, value interface{}) error {
	rawBytes, err := r.c.Get(ctx, r.bh)
	if err != nil {
		return err
	}
	return rawBytes.ToValue(value)
}

// Put implements Row.Put.
func (r *row) Put(ctx *context.T, value interface{}) error {
	rawBytes, err := vom.RawBytesFromValue(value)
	if err != nil {
		return err
	}
	return r.c.Put(ctx, r.bh, rawBytes)
}

// Delete implements Row.Delete.
func (r *row) Delete(ctx *context.T) error {
	return r.c.Delete(ctx, r.bh)
}
