// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"unsafe"

	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/ref/services/syncbase/discovery"
)

// All "x.extract" methods return a native Go type and leave x in the same state
// as "x.free". The "x.free" methods are idempotent. A counterpart set of
// "x.extractToJava" methods are in jni_types.go and jni.go.

/*
#include <stdlib.h>
#include <string.h>
#include "lib.h"
*/
import "C"

////////////////////////////////////////////////////////////
// C.v23_syncbase_BatchOptions

func (x *C.v23_syncbase_BatchOptions) init(opts wire.BatchOptions) {
	x.hint.init(opts.Hint)
	x.readOnly = C.bool(opts.ReadOnly)
}

func (x *C.v23_syncbase_BatchOptions) extract() wire.BatchOptions {
	return wire.BatchOptions{
		Hint:     x.hint.extract(),
		ReadOnly: bool(x.readOnly),
	}
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_Bool

func (x *C.v23_syncbase_Bool) init(b bool) {
	if b == false {
		*x = 0
	} else {
		*x = 1
	}
}

func (x *C.v23_syncbase_Bool) extract() bool {
	if *x == 0 {
		return false
	}
	return true
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_Bytes

func init() {
	if C.sizeof_uint8_t != 1 {
		panic(C.sizeof_uint8_t)
	}
}

func (x *C.v23_syncbase_Bytes) init(b []byte) {
	// Special-case for len(b) == 0, because memcpy fails on invalid pointers even if size is 0.
	if len(b) == 0 {
		x.n = 0
		x.p = nil
		return
	}
	x.n = C.int(len(b))
	x.p = (*C.uint8_t)(C.malloc(C.size_t(x.n)))
	C.memcpy(unsafe.Pointer(x.p), unsafe.Pointer(&b[0]), C.size_t(x.n))
}

func (x *C.v23_syncbase_Bytes) extract() []byte {
	if x.p == nil {
		return nil
	}
	res := C.GoBytes(unsafe.Pointer(x.p), x.n)
	C.free(unsafe.Pointer(x.p))
	x.p = nil
	x.n = 0
	return res
}

func (x *C.v23_syncbase_Bytes) free() {
	if x.p != nil {
		C.free(unsafe.Pointer(x.p))
		x.p = nil
	}
	x.n = 0
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_CollectionRowPattern

func (x *C.v23_syncbase_CollectionRowPattern) init(crp wire.CollectionRowPattern) {
	x.collectionBlessing.init(crp.CollectionBlessing)
	x.collectionName.init(crp.CollectionName)
	x.rowKey.init(crp.RowKey)
}

func (x *C.v23_syncbase_CollectionRowPattern) extract() wire.CollectionRowPattern {
	return wire.CollectionRowPattern{
		CollectionBlessing: x.collectionBlessing.extract(),
		CollectionName:     x.collectionName.extract(),
		RowKey:             x.rowKey.extract(),
	}
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_CollectionRowPatterns

func (x *C.v23_syncbase_CollectionRowPatterns) at(i int) *C.v23_syncbase_CollectionRowPattern {
	return (*C.v23_syncbase_CollectionRowPattern)(unsafe.Pointer(uintptr(unsafe.Pointer(x.p)) + uintptr(C.size_t(i)*C.sizeof_v23_syncbase_CollectionRowPattern)))
}

func (x *C.v23_syncbase_CollectionRowPatterns) init(crps []wire.CollectionRowPattern) {
	x.n = C.int(len(crps))
	x.p = (*C.v23_syncbase_CollectionRowPattern)(C.malloc(C.size_t(x.n) * C.sizeof_v23_syncbase_CollectionRowPattern))
	for i, v := range crps {
		x.at(i).init(v)
	}
}

func (x *C.v23_syncbase_CollectionRowPatterns) extract() []wire.CollectionRowPattern {
	if x.p == nil {
		return nil
	}
	res := make([]wire.CollectionRowPattern, x.n)
	for i := 0; i < int(x.n); i++ {
		res[i] = x.at(i).extract()
	}
	C.free(unsafe.Pointer(x.p))
	x.p = nil
	x.n = 0
	return res
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_Id

func (x *C.v23_syncbase_Id) init(id wire.Id) {
	x.blessing.init(id.Blessing)
	x.name.init(id.Name)
}

func (x *C.v23_syncbase_Id) toId() wire.Id {
	return wire.Id{
		Blessing: x.blessing.extract(),
		Name:     x.name.extract(),
	}
}

func (x *C.v23_syncbase_Id) free() {
	x.blessing.free()
	x.name.free()
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_Ids

func (x *C.v23_syncbase_Ids) at(i int) *C.v23_syncbase_Id {
	return (*C.v23_syncbase_Id)(unsafe.Pointer(uintptr(unsafe.Pointer(x.p)) + uintptr(C.size_t(i)*C.sizeof_v23_syncbase_Id)))
}

func (x *C.v23_syncbase_Ids) init(ids []wire.Id) {
	x.n = C.int(len(ids))
	x.p = (*C.v23_syncbase_Id)(C.malloc(C.size_t(x.n) * C.sizeof_v23_syncbase_Id))
	for i, v := range ids {
		x.at(i).init(v)
	}
}

func (x *C.v23_syncbase_Ids) extract() []wire.Id {
	if x.p == nil {
		return nil
	}
	res := make([]wire.Id, x.n)
	for i := 0; i < int(x.n); i++ {
		res[i] = x.at(i).toId()
	}
	C.free(unsafe.Pointer(x.p))
	x.p = nil
	x.n = 0
	return res
}

func (x *C.v23_syncbase_Ids) free() {
	if x.p == nil {
		return
	}
	for i := 0; i < int(x.n); i++ {
		x.at(i).free()
	}
	C.free(unsafe.Pointer(x.p))
	x.p = nil
	x.n = 0
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_KeyValue

func (x *C.v23_syncbase_KeyValue) init(key string, value []byte) {
	x.key.init(key)
	x.value.init(value)
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_Permissions

func (x *C.v23_syncbase_Permissions) init(perms access.Permissions) {
	b := new(bytes.Buffer)
	if err := access.WritePermissions(b, perms); err != nil {
		panic(err)
	}
	x.json.init(b.Bytes())
}

func (x *C.v23_syncbase_Permissions) extract() access.Permissions {
	b := x.json.extract()
	if len(b) == 0 {
		return nil
	}
	perms, err := access.ReadPermissions(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return perms
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_String

func (x *C.v23_syncbase_String) init(s string) {
	x.n = C.int(len(s))
	x.p = C.CString(s)
}

func (x *C.v23_syncbase_String) extract() string {
	if x.p == nil {
		return ""
	}
	res := C.GoStringN(x.p, x.n)
	C.free(unsafe.Pointer(x.p))
	x.p = nil
	x.n = 0
	return res
}

func (x *C.v23_syncbase_String) free() {
	if x.p != nil {
		C.free(unsafe.Pointer(x.p))
		x.p = nil
	}
	x.n = 0
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_Strings

func (x *C.v23_syncbase_Strings) at(i int) *C.v23_syncbase_String {
	return (*C.v23_syncbase_String)(unsafe.Pointer(uintptr(unsafe.Pointer(x.p)) + uintptr(C.size_t(i)*C.sizeof_v23_syncbase_String)))
}

func (x *C.v23_syncbase_Strings) init(strs []string) {
	x.n = C.int(len(strs))
	x.p = (*C.v23_syncbase_String)(C.malloc(C.size_t(x.n) * C.sizeof_v23_syncbase_String))
	for i, v := range strs {
		x.at(i).init(v)
	}
}

func (x *C.v23_syncbase_Strings) extract() []string {
	if x.p == nil {
		return nil
	}
	res := make([]string, x.n)
	for i := 0; i < int(x.n); i++ {
		res[i] = x.at(i).extract()
	}
	C.free(unsafe.Pointer(x.p))
	x.p = nil
	x.n = 0
	return res
}

func (x *C.v23_syncbase_Strings) free() {
	if x.p == nil {
		return
	}
	for i := 0; i < int(x.n); i++ {
		x.at(i).free()
	}
	C.free(unsafe.Pointer(x.p))
	x.p = nil
	x.n = 0
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_SyncgroupSpec

func (x *C.v23_syncbase_SyncgroupSpec) init(spec wire.SyncgroupSpec) {
	x.description.init(spec.Description)
	x.publishSyncbaseName.init(spec.PublishSyncbaseName)
	x.perms.init(spec.Perms)
	x.collections.init(spec.Collections)
	x.mountTables.init(spec.MountTables)
	x.isPrivate = C.bool(spec.IsPrivate)
}

func (x *C.v23_syncbase_SyncgroupSpec) toSyncgroupSpec() wire.SyncgroupSpec {
	return wire.SyncgroupSpec{
		Description:         x.description.extract(),
		PublishSyncbaseName: x.publishSyncbaseName.extract(),
		Perms:               x.perms.extract(),
		Collections:         x.collections.extract(),
		MountTables:         x.mountTables.extract(),
		IsPrivate:           bool(x.isPrivate),
	}
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_SyncgroupMemberInfo

func (x *C.v23_syncbase_SyncgroupMemberInfo) init(member wire.SyncgroupMemberInfo) {
	x.syncPriority = C.uint8_t(member.SyncPriority)
	x.blobDevType = C.uint8_t(member.BlobDevType)
}

func (x *C.v23_syncbase_SyncgroupMemberInfo) toSyncgroupMemberInfo() wire.SyncgroupMemberInfo {
	return wire.SyncgroupMemberInfo{
		SyncPriority: byte(x.syncPriority),
		BlobDevType:  byte(x.blobDevType),
	}
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_SyncgroupMemberInfoMap

func (x *C.v23_syncbase_SyncgroupMemberInfoMap) at(i int) (*C.v23_syncbase_String, *C.v23_syncbase_SyncgroupMemberInfo) {
	k := (*C.v23_syncbase_String)(unsafe.Pointer(uintptr(unsafe.Pointer(x.keys)) + uintptr(C.size_t(i)*C.sizeof_v23_syncbase_String)))
	v := (*C.v23_syncbase_SyncgroupMemberInfo)(unsafe.Pointer(uintptr(unsafe.Pointer(x.values)) + uintptr(C.size_t(i)*C.sizeof_v23_syncbase_SyncgroupMemberInfo)))
	return k, v
}

func (x *C.v23_syncbase_SyncgroupMemberInfoMap) init(members map[string]wire.SyncgroupMemberInfo) {
	x.n = C.int(len(members))
	x.keys = (*C.v23_syncbase_String)(C.malloc(C.size_t(x.n) * C.sizeof_v23_syncbase_String))
	x.values = (*C.v23_syncbase_SyncgroupMemberInfo)(C.malloc(C.size_t(x.n) * C.sizeof_v23_syncbase_SyncgroupMemberInfo))
	i := 0
	for k, v := range members {
		ck, cv := x.at(i)
		ck.init(k)
		cv.init(v)
		i++
	}
}

func (x *C.v23_syncbase_SyncgroupMemberInfoMap) free() {
	if x.n == 0 {
		return
	}
	if x.keys == nil {
		panic("v23_syncbase_SyncgroupMemberInfoMap keys corruption")
	}
	if x.values == nil {
		panic("v23_syncbase_SyncgroupMemberInfoMap values corruption")
	}
	for i := 0; i < int(x.n); i++ {
		// The values don't contain pointers.
		k, _ := x.at(i)
		k.free()
	}
	C.free(unsafe.Pointer(x.keys))
	C.free(unsafe.Pointer(x.values))
	x.keys = nil
	x.values = nil
	x.n = 0
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_VError

func (x *C.v23_syncbase_VError) init(err error) {
	if err == nil {
		return
	}
	x.id.init(string(verror.ErrorID(err)))
	x.actionCode = C.uint(verror.Action(err))
	x.msg.init(err.Error())
	x.stack.init(verror.Stack(err).String())
}

func (x *C.v23_syncbase_VError) free() {
	x.id.free()
	x.actionCode = C.uint(0)
	x.msg.free()
	x.stack.free()
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_WatchChange

func (x *C.v23_syncbase_WatchChange) init(wc syncbase.WatchChange) error {
	x.entityType = C.v23_syncbase_EntityType(wc.EntityType)
	x.collection.init(wc.Collection)
	x.row.init(wc.Row)
	x.changeType = C.v23_syncbase_ChangeType(wc.ChangeType)
	var value []byte
	switch wc.EntityType {
	case syncbase.EntityRoot:
		// Nothing to do.
	case syncbase.EntityCollection:
		// TODO(razvanm): Pass the CollectionInfo from wc.value.
	case syncbase.EntityRow:
		if wc.ChangeType != syncbase.DeleteChange {
			var valueAsRawBytes vom.RawBytes
			var err error
			if err = wc.Value(&valueAsRawBytes); err != nil {
				return err
			}
			if clientUnderstandsVOM {
				value, err = vom.Encode(valueAsRawBytes)
			} else {
				err = valueAsRawBytes.ToValue(&value)
			}
			if err != nil {
				return err
			}
		}
	}
	x.value.init(value)
	x.resumeMarker.init(wc.ResumeMarker)
	x.fromSync = C.bool(wc.FromSync)
	x.continued = C.bool(wc.Continued)
	return nil
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_Invite

func (x *C.v23_syncbase_Invite) init(invite discovery.Invite) {
	x.syncgroup.init(invite.Syncgroup)
	x.addresses.init(invite.Addresses)
	x.blessingNames.init(invite.BlessingNames)
}

////////////////////////////////////////////////////////////
// C.v23_syncbase_AppPeer

func (x *C.v23_syncbase_AppPeer) init(peer discovery.AppPeer) {
	x.appName.init(peer.AppName)
	x.blessings.init(peer.Blessings)
	x.isLost = C.bool(peer.Lost)
}
