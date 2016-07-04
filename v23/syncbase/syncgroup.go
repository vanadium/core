// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
)

var (
	_ Syncgroup = (*syncgroup)(nil)
)

type syncgroup struct {
	c  wire.DatabaseClientMethods
	id wire.Id
}

func newSyncgroup(dbName string, id wire.Id) Syncgroup {
	return &syncgroup{
		c:  wire.DatabaseClient(dbName),
		id: id,
	}
}

// Create implements Syncgroup.Create.
func (sg *syncgroup) Create(ctx *context.T, spec wire.SyncgroupSpec, myInfo wire.SyncgroupMemberInfo) error {
	return sg.c.CreateSyncgroup(ctx, sg.id, spec, myInfo)
}

// Join implements Syncgroup.Join.
func (sg *syncgroup) Join(ctx *context.T, remoteSyncbaseName string, expectedSyncbaseBlessings []string, myInfo wire.SyncgroupMemberInfo) (wire.SyncgroupSpec, error) {
	return sg.c.JoinSyncgroup(ctx, remoteSyncbaseName, expectedSyncbaseBlessings, sg.id, myInfo)
}

// Leave implements Syncgroup.Leave.
func (sg *syncgroup) Leave(ctx *context.T) error {
	return sg.c.LeaveSyncgroup(ctx, sg.id)
}

// Destroy implements Syncgroup.Destroy.
func (sg *syncgroup) Destroy(ctx *context.T) error {
	return sg.c.DestroySyncgroup(ctx, sg.id)
}

// Eject implements Syncgroup.Eject.
func (sg *syncgroup) Eject(ctx *context.T, member string) error {
	return sg.c.EjectFromSyncgroup(ctx, sg.id, member)
}

// GetSpec implements Syncgroup.GetSpec.
func (sg *syncgroup) GetSpec(ctx *context.T) (wire.SyncgroupSpec, string, error) {
	return sg.c.GetSyncgroupSpec(ctx, sg.id)
}

// SetSpec implements Syncgroup.SetSpec.
func (sg *syncgroup) SetSpec(ctx *context.T, spec wire.SyncgroupSpec, version string) error {
	return sg.c.SetSyncgroupSpec(ctx, sg.id, spec, version)
}

// GetMembers implements Syncgroup.GetMembers.
func (sg *syncgroup) GetMembers(ctx *context.T) (map[string]wire.SyncgroupMemberInfo, error) {
	return sg.c.GetSyncgroupMembers(ctx, sg.id)
}

// Id implements Syncgroup.Id.
func (sg *syncgroup) Id() wire.Id {
	return sg.id
}
