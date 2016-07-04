// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
)

func newCollection(parentFullName string, id wire.Id, bh wire.BatchHandle) Collection {
	fullName := naming.Join(parentFullName, util.EncodeId(id))
	return &collection{
		c:        wire.CollectionClient(fullName),
		fullName: fullName,
		id:       id,
		bh:       bh,
	}
}

type collection struct {
	c        wire.CollectionClientMethods
	fullName string
	id       wire.Id
	bh       wire.BatchHandle
}

var _ Collection = (*collection)(nil)

// Id implements Collection.Id.
func (c *collection) Id() wire.Id {
	return c.id
}

// FullName implements Collection.FullName.
func (c *collection) FullName() string {
	return c.fullName
}

// Exists implements Collection.Exists.
func (c *collection) Exists(ctx *context.T) (bool, error) {
	return c.c.Exists(ctx, c.bh)
}

// Create implements Collection.Create.
func (c *collection) Create(ctx *context.T, perms access.Permissions) error {
	if perms == nil {
		// Default to giving full permissions to the creator.
		_, user, err := util.AppAndUserPatternFromBlessings(security.DefaultBlessingNames(v23.GetPrincipal(ctx))...)
		if err != nil {
			return verror.New(wire.ErrInferDefaultPermsFailed, ctx, "Collection", util.EncodeId(c.id), err)
		}
		perms = access.Permissions{}.Add(user, access.TagStrings(wire.AllCollectionTags...)...)
	}
	return c.c.Create(ctx, c.bh, perms)
}

// Destroy implements Collection.Destroy.
func (c *collection) Destroy(ctx *context.T) error {
	return c.c.Destroy(ctx, c.bh)
}

// GetPermissions implements Collection.GetPermissions.
func (c *collection) GetPermissions(ctx *context.T) (access.Permissions, error) {
	return c.c.GetPermissions(ctx, c.bh)
}

// SetPermissions implements Collection.SetPermissions.
func (c *collection) SetPermissions(ctx *context.T, perms access.Permissions) error {
	return c.c.SetPermissions(ctx, c.bh, perms)
}

// Row implements Collection.Row.
func (c *collection) Row(key string) Row {
	return newRow(c.fullName, key, c.bh)
}

// Get implements Collection.Get.
func (c *collection) Get(ctx *context.T, key string, value interface{}) error {
	return c.Row(key).Get(ctx, value)
}

// Put implements Collection.Put.
func (c *collection) Put(ctx *context.T, key string, value interface{}) error {
	return c.Row(key).Put(ctx, value)
}

// Delete implements Collection.Delete.
func (c *collection) Delete(ctx *context.T, key string) error {
	return c.Row(key).Delete(ctx)
}

// DeleteRange implements Collection.DeleteRange.
func (c *collection) DeleteRange(ctx *context.T, r RowRange) error {
	return c.c.DeleteRange(ctx, c.bh, []byte(r.Start()), []byte(r.Limit()))
}

// Scan implements Collection.Scan.
func (c *collection) Scan(ctx *context.T, r RowRange) ScanStream {
	ctx, cancel := context.WithCancel(ctx)
	call, err := c.c.Scan(ctx, c.bh, []byte(r.Start()), []byte(r.Limit()))
	if err != nil {
		cancel()
		return &invalidScanStream{invalidStream{err: err}}
	}
	return newScanStream(cancel, call)
}
