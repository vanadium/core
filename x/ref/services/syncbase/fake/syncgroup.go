// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
)

// syncgroup implements the syncbase.Syncgroup interface.
type syncgroup struct {
	common
	specErr error
}

func (*syncgroup) Create(*context.T, wire.SyncgroupSpec, wire.SyncgroupMemberInfo) error {
	return nil
}
func (*syncgroup) Join(
	*context.T, string, []string, wire.SyncgroupMemberInfo) (wire.SyncgroupSpec, error,
) {
	return wire.SyncgroupSpec{}, nil
}
func (*syncgroup) Leave(*context.T) error         { return nil }
func (*syncgroup) Eject(*context.T, string) error { return nil }
func (sg *syncgroup) GetSpec(*context.T) (wire.SyncgroupSpec, string, error) {
	return wire.SyncgroupSpec{}, "", sg.specErr
}
func (*syncgroup) SetSpec(*context.T, wire.SyncgroupSpec, string) error {
	return nil
}
func (*syncgroup) GetMembers(*context.T) (map[string]wire.SyncgroupMemberInfo, error) {
	return nil, nil
}
