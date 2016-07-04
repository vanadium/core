// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build mojo

// NOTE(sadovsky): The code below reflects some, but not all, of the Syncbase
// API changes and simplifications. If at some point we choose to update this
// code, the person doing this work should compare against bridge/cgo/impl.go to
// figure out what needs to change.

// Implementation of Syncbase Mojo stubs. Our strategy is to translate Mojo stub
// requests into Vanadium stub requests, and Vanadium stub responses into Mojo
// stub responses. As part of this procedure, we synthesize "fake" ctx and call
// objects to pass to the Vanadium stubs.
//
// Implementation notes:
// - This API partly mirrors the Syncbase RPC API. Many methods take 'name' as
//   their first argument; this is a service-relative Vanadium object name. For
//   example, the 'name' argument to DbCreate is an encoded database id.
// - Variables with Mojo-specific types have names that start with "m".

package bridge_mojo

import (
	"bytes"
	"strings"

	"mojo/public/go/bindings"
	mojom "mojom/syncbase"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/v23/services/permissions"
	wire "v.io/v23/services/syncbase"
	watchwire "v.io/v23/services/watch"
	"v.io/v23/syncbase/util"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/ref/services/syncbase/bridge"
)

// Global state, initialized by MojoMain.
type mojoImpl struct {
	bridge.Bridge
}

func NewMojoImpl(ctx *context.T, srv rpc.Server, disp rpc.Dispatcher) *mojoImpl {
	return &mojoImpl{Bridge: bridge.NewBridge(ctx, srv, disp)}
}

////////////////////////////////////////
// Struct converters

func toMojoError(err error) mojom.Error {
	if err == nil {
		return mojom.Error{}
	}
	return mojom.Error{
		Id:         string(verror.ErrorID(err)),
		ActionCode: uint32(verror.Action(err)),
		Msg:        err.Error(),
	}
}

func toV23Id(mId mojo.Id) wire.Id {
	return wire.Id{Blessing: mId.Blessing, Name: mId.Name}
}

func toMojoId(id wire.Id) mojo.Id {
	return mojo.Id{Blessing: id.Blessing, Name: id.Name}
}

func toV23Ids(mIds []mojo.Id) []wire.Id {
	res := make([]wire.Id, len(ids))
	for i, v := range ids {
		res[i] = toV23Id(v)
	}
	return res
}

func toMojoIds(ids []wire.Id) []mojo.Id {
	res := make([]mojo.Id, len(ids))
	for i, v := range ids {
		res[i] = toMojoId(v)
	}
	return res
}

func toV23Perms(mPerms mojom.Perms) (access.Permissions, error) {
	return access.ReadPermissions(strings.NewReader(mPerms.Json))
}

func toMojoPerms(perms access.Permissions) (mojom.Perms, error) {
	b := new(bytes.Buffer)
	if err := access.WritePermissions(b, perms); err != nil {
		return mojom.Perms{}, err
	}
	return mojom.Perms{Json: b.String()}, nil
}

func toV23SyncgroupMemberInfo(mInfo mojom.SyncgroupMemberInfo) wire.SyncgroupMemberInfo {
	return wire.SyncgroupMemberInfo{
		SyncPriority: mInfo.SyncPriority,
		BlobDevType:  mInfo.BlobDevType,
	}
}

func toMojoSyncgroupMemberInfo(info wire.SyncgroupMemberInfo) mojom.SyncgroupMemberInfo {
	return mojom.SyncgroupMemberInfo{
		SyncPriority: info.SyncPriority,
		BlobDevType:  Info.BlobDevType,
	}
}

func toV23SyncgroupSpec(mSpec mojom.SyncgroupSpec) (wire.SyncgroupSpec, error) {
	perms, err := toV23Perms(mSpec.Perms)
	if err != nil {
		return wire.SyncgroupSpec{}, err
	}
	return wire.SyncgroupSpec{
		Description: mSpec.Description,
		Perms:       perms,
		Collections: toV23Ids(mSpec.Collections),
		MountTables: mSpec.MountTables,
		IsPrivate:   mSpec.IsPrivate,
	}, nil
}

func toMojoSyncgroupSpec(spec wire.SyncgroupSpec) (mojom.SyncgroupSpec, error) {
	mPerms, err := toMojoPerms(spec.Perms)
	if err != nil {
		return mojom.SyncgroupSpec{}, err
	}
	return mojom.SyncgroupSpec{
		Description: spec.Description,
		Perms:       mPerms,
		Collections: toMojoIds(spec.Collections),
		MountTables: spec.MountTables,
		IsPrivate:   spec.IsPrivate,
	}, nil
}

func toV23BatchOptions(mOpts mojom.BatchOptions) wire.BatchOptions {
	return wire.BatchOptions{
		Hint:     mOpts.Hint,
		ReadOnly: mOpts.ReadOnly,
	}
}

////////////////////////////////////////
// Glob utils

func (m *mojoImpl) listChildIds(name string) (mojom.Error, []mojom.Id, error) {
	ctx, call := m.NewCtxCall(name, rpc.MethodDesc{
		Name: "GlobChildren__",
	})
	stub, err := m.GetGlobber(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil, nil
	}
	gcsCall := &globChildrenServerCall{call, ctx, make([]wire.Id, 0)}
	g, err := glob.Parse("*")
	if err != nil {
		return toMojoError(err), nil, nil
	}
	if err := stub.GlobChildren__(ctx, gcsCall, g.Head()); err != nil {
		return toMojoError(err), nil, nil
	}
	return toMojoError(nil), toMojoIds(gcsCall.Ids), nil
}

type globChildrenServerCall struct {
	rpc.ServerCall
	ctx *context.T
	Ids []wire.Id
}

func (g *globChildrenServerCall) SendStream() interface {
	Send(naming.GlobChildrenReply) error
} {
	return g
}

func (g *globChildrenServerCall) Send(reply naming.GlobChildrenReply) error {
	switch v := reply.(type) {
	case *naming.GlobChildrenReplyName:
		encId := v.Value[strings.LastIndex(v.Value, "/")+1:]
		// Component ids within object names are always encoded. See comment in
		// server/dispatcher.go for explanation.
		id, err := util.DecodeId(encId)
		if err != nil {
			// If this happens, there's a bug in the Syncbase server. Glob should
			// return names with escaped components.
			return verror.New(verror.ErrInternal, nil, err)
		}
		g.Ids = append(g.Ids, id)
	case *naming.GlobChildrenReplyError:
		return verror.New(verror.ErrInternal, nil, v.Value.Error)
	}
	return nil
}

////////////////////////////////////////
// Service

// TODO(sadovsky): All Mojo stub implementations return a nil error (the last
// return value), since that error doesn't make it back to the IPC client. Chat
// with rogulenko@ about whether we should change the Go Mojo stub generator to
// drop these errors.
func (m *mojoImpl) ServiceGetPermissions() (mojom.Error, mojom.Perms, string, error) {
	ctx, call := m.NewCtxCall("", bridge.MethodDesc(permissions.ObjectDesc, "GetPermissions"))
	stub, err := m.GetService(ctx, call)
	if err != nil {
		return toMojoError(err), mojom.Perms{}, "", nil
	}
	perms, version, err := stub.GetPermissions(ctx, call)
	if err != nil {
		return toMojoError(err), mojom.Perms{}, "", nil
	}
	mPerms, err := toMojoPerms(perms)
	if err != nil {
		return toMojoError(err), mojom.Perms{}, "", nil
	}
	return toMojoError(err), mPerms, version, nil
}

func (m *mojoImpl) ServiceSetPermissions(mPerms mojom.Perms, version string) (mojom.Error, error) {
	ctx, call := m.NewCtxCall("", bridge.MethodDesc(permissions.ObjectDesc, "SetPermissions"))
	stub, err := m.GetService(ctx, call)
	if err != nil {
		return toMojoError(err), nil
	}
	perms, err := toV23Perms(mPerms)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.SetPermissions(ctx, call, perms, version)
	return toMojoError(err), nil
}

func (m *mojoImpl) ServiceListDatabases() (mojom.Error, []mojom.Id, error) {
	return m.listChildren("")
}

////////////////////////////////////////
// Database

func (m *mojoImpl) DbCreate(name string, mPerms mojom.Perms) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Create"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	perms, err := toV23Perms(mPerms)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.Create(ctx, call, nil, perms)
	return toMojoError(err), nil
}

func (m *mojoImpl) DbDestroy(name string) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Destroy"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.Destroy(ctx, call)
	return toMojoError(err), nil
}

func (m *mojoImpl) DbExists(name string) (mojom.Error, bool, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Exists"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), false, nil
	}
	exists, err := stub.Exists(ctx, call)
	return toMojoError(err), exists, nil
}

func (m *mojoImpl) DbListCollections(name, batchHandle string) (mojom.Error, []mojom.Id, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "ListCollections"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil, nil
	}
	ids, err := stub.ListCollections(ctx, call, batchHandle)
	return toMojoError(err), ids, nil
}

type execStreamImpl struct {
	ctx   *context.T
	proxy *mojom.ExecStream_Proxy
}

func (s *execStreamImpl) Send(item interface{}) error {
	rb, ok := item.([]*vom.RawBytes)
	if !ok {
		return verror.NewErrInternal(s.ctx)
	}

	// TODO(aghassemi): Switch generic values on the wire from '[]byte' to 'any'.
	// Currently, exec is the only method that uses 'any' rather than '[]byte'.
	// https://github.com/vanadium/issues/issues/766
	var values [][]byte
	for _, raw := range rb {
		var bytes []byte
		// The value type can be either string (for column headers and row keys) or
		// []byte (for values).
		if raw.Type.Kind() == vdl.String {
			var str string
			if err := raw.ToValue(&str); err != nil {
				return err
			}
			bytes = []byte(str)
		} else {
			if err := raw.ToValue(&bytes); err != nil {
				return err
			}
		}
		values = append(values, bytes)
	}

	r := mojom.Result{
		Values: values,
	}

	// proxy.OnResult() blocks until the client acks the previous invocation,
	// thus providing flow control.
	return s.proxy.OnResult(r)
}

func (s *execStreamImpl) Recv(_ interface{}) error {
	// This should never be called.
	return verror.NewErrInternal(s.ctx)
}

var _ rpc.Stream = (*execStreamImpl)(nil)

func (m *mojoImpl) DbExec(name, batchHandle, query string, params [][]byte, ptr mojom.ExecStream_Pointer) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Exec"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}

	// TODO(ivanpi): For now, Dart always gives us []byte, and here we convert
	// []byte to vom.RawBytes as required by Exec. This will need to change once
	// we support VDL/VOM in Dart.
	// https://github.com/vanadium/issues/issues/766
	paramsVom := make([]*vom.RawBytes, len(params))
	for i, p := range params {
		var err error
		if paramsVom[i], err = vom.RawBytesFromValue(p); err != nil {
			return toMojoError(err), nil
		}
	}

	proxy := mojom.NewExecStreamProxy(ptr, bindings.GetAsyncWaiter())

	execServerCallStub := &wire.DatabaseExecServerCallStub{struct {
		rpc.Stream
		rpc.ServerCall
	}{
		&execStreamImpl{
			ctx:   ctx,
			proxy: proxy,
		},
		call,
	}}

	go func() {
		var err = stub.Exec(ctx, execServerCallStub, batchHandle, query, paramsVom)
		// NOTE(nlacasse): Since we are already streaming, we send any error back
		// to the client on the stream.  The Exec function itself should not
		// return an error at this point.
		proxy.OnDone(toMojoError(err))
	}()

	return mojom.Error{}, nil
}

func (m *mojoImpl) DbBeginBatch(name string, mOpts mojom.BatchOptions) (mojom.Error, string, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "BeginBatch"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), "", nil
	}
	batchHandle, err := stub.BeginBatch(ctx, call, toV23BatchOptions(mOpts))
	return toMojoError(err), batchHandle, nil
}

func (m *mojoImpl) DbCommit(name, batchHandle string) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Commit"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.Commit(ctx, call, batchHandle)
	return toMojoError(err), nil
}

func (m *mojoImpl) DbAbort(name, batchHandle string) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Abort"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.Abort(ctx, call, batchHandle)
	return toMojoError(err), nil
}

func (m *mojoImpl) DbGetPermissions(name string) (mojom.Error, mojom.Perms, string, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(permissions.ObjectDesc, "GetPermissions"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), mojom.Perms{}, "", nil
	}
	perms, version, err := stub.GetPermissions(ctx, call)
	if err != nil {
		return toMojoError(err), mojom.Perms{}, "", nil
	}
	mPerms, err := toMojoPerms(perms)
	if err != nil {
		return toMojoError(err), mojom.Perms{}, "", nil
	}
	return toMojoError(err), mPerms, version, nil
}

func (m *mojoImpl) DbSetPermissions(name string, mPerms mojom.Perms, version string) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(permissions.ObjectDesc, "SetPermissions"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	perms, err := toV23Perms(mPerms)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.SetPermissions(ctx, call, perms, version)
	return toMojoError(err), nil
}

type watchGlobStreamImpl struct {
	ctx   *context.T
	proxy *mojom.WatchGlobStream_Proxy
}

func (s *watchGlobStreamImpl) Send(item interface{}) error {
	c, ok := item.(watchwire.Change)
	if !ok {
		return verror.NewErrInternal(s.ctx)
	}
	vc := syncbase.ToWatchChange(c)

	var value []byte
	if vc.ChangeType != DeleteChange {
		if err := vc.Value(&value); err != nil {
			return err
		}
	}
	mc := mojom.WatchChange{
		CollectionName: vc.Collection,
		RowKey:         vc.Row,
		ChangeType:     uint32(vc.ChangeType),
		ValueBytes:     value,
		ResumeMarker:   vc.ResumeMarker,
		FromSync:       vc.FromSync,
		Continued:      vc.Continued,
	}

	// proxy.OnChange() blocks until the client acks the previous invocation,
	// thus providing flow control.
	return s.proxy.OnChange(mc)
}

func (s *watchGlobStreamImpl) Recv(_ interface{}) error {
	// This should never be called.
	return verror.NewErrInternal(s.ctx)
}

var _ rpc.Stream = (*watchGlobStreamImpl)(nil)

func (m *mojoImpl) DbWatchGlob(name string, mReq mojom.GlobRequest, ptr mojom.WatchGlobStream_Pointer) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(watchwire.GlobWatcherDesc, "WatchGlob"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}

	var vReq = watchwire.GlobRequest{
		Pattern:      mReq.Pattern,
		ResumeMarker: watchwire.ResumeMarker(mReq.ResumeMarker),
	}
	proxy := mojom.NewWatchGlobStreamProxy(ptr, bindings.GetAsyncWaiter())

	watchGlobServerCallStub := &watchwire.GlobWatcherWatchGlobServerCallStub{struct {
		rpc.Stream
		rpc.ServerCall
	}{
		&watchGlobStreamImpl{
			ctx:   ctx,
			proxy: proxy,
		},
		call,
	}}

	go func() {
		var err = stub.WatchGlob(ctx, watchGlobServerCallStub, vReq)
		// NOTE(nlacasse): Since we are already streaming, we send any error back
		// to the client on the stream.  The WatchGlob function itself should not
		// return an error at this point.
		// NOTE(aghassemi): WatchGlob does not terminate unless there is an error.
		proxy.OnError(toMojoError(err))
	}()

	return mojom.Error{}, nil
}

func (m *mojoImpl) DbGetResumeMarker(name, batchHandle string) (mojom.Error, []byte, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseWatcherDesc, "GetResumeMarker"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil, nil
	}
	marker, err := stub.GetResumeMarker(ctx, call, batchHandle)
	return toMojoError(err), marker, nil
}

////////////////////////////////////////
// SyncgroupManager

func (m *mojoImpl) DbListSyncgroups(name string) (mojom.Error, []mojo.Id, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "GetSyncgroupNames"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil, nil
	}
	ids, err := stub.ListSyncgroups(ctx, call)
	return toMojoError(err), toMojoIds(ids), nil
}

func (m *mojoImpl) DbCreateSyncgroup(name string, sgId mojo.Id, spec mojom.SyncgroupSpec, myInfo mojom.SyncgroupMemberInfo) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "CreateSyncgroup"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	v23Spec, err := toV23SyncgroupSpec(spec)
	if err != nil {
		return toMojoError(err), nil
	}
	return toMojoError(stub.CreateSyncgroup(ctx, call, toV23Id(sgId), v23Spec, toV23SyncgroupMemberInfo(myInfo))), nil
}

func (m *mojoImpl) DbJoinSyncgroup(name string, sgId mojo.Id, myInfo mojom.SyncgroupMemberInfo) (mojom.Error, mojom.SyncgroupSpec, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "JoinSyncgroup"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), mojom.SyncgroupSpec{}, nil
	}
	spec, err := stub.JoinSyncgroup(ctx, call, toV23Id(sgId), toV23SyncgroupMemberInfo(myInfo))
	if err != nil {
		return toMojoError(err), mojom.SyncgroupSpec{}, nil
	}
	mojoSpec, err := toMojoSyncgroupSpec(spec)
	if err != nil {
		return toMojoError(err), mojom.SyncgroupSpec{}, nil
	}
	return toMojoError(err), mojoSpec, nil
}

func (m *mojoImpl) DbLeaveSyncgroup(name string, sgId mojo.Id) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "LeaveSyncgroup"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	return toMojoError(stub.LeaveSyncgroup(ctx, call, toV23Id(sgId))), nil
}

func (m *mojoImpl) DbDestroySyncgroup(name string, sgId mojo.Id) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "DestroySyncgroup"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	return toMojoError(stub.DestroySyncgroup(ctx, call, toV23Id(sgId))), nil
}

func (m *mojoImpl) DbEjectFromSyncgroup(name string, sgId mojo.Id, member string) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "EjectFromSyncgroup"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	return toMojoError(stub.EjectFromSyncgroup(ctx, call, toV23Id(sgId), member)), nil
}

func (m *mojoImpl) DbGetSyncgroupSpec(name string, sgId mojo.Id) (mojom.Error, mojom.SyncgroupSpec, string, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "GetSyncgroupSpec"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), mojom.SyncgroupSpec{}, "", nil
	}
	spec, version, err := stub.GetSyncgroupSpec(ctx, call, toV23Id(sgId))
	mojoSpec, err := toMojoSyncgroupSpec(spec)
	if err != nil {
		return toMojoError(err), mojom.SyncgroupSpec{}, "", nil
	}
	return toMojoError(err), mojoSpec, version, nil
}

func (m *mojoImpl) DbSetSyncgroupSpec(name string, sgId mojo.Id, spec mojom.SyncgroupSpec, version string) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "SetSyncgroupSpec"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	v23Spec, err := toV23SyncgroupSpec(spec)
	if err != nil {
		return toMojoError(err), nil
	}
	return toMojoError(stub.SetSyncgroupSpec(ctx, call, toV23Id(sgId), v23Spec, version)), nil
}

func (m *mojoImpl) DbGetSyncgroupMembers(name string, sgId mojo.Id) (mojom.Error, map[string]mojom.SyncgroupMemberInfo, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "GetSyncgroupMembers"))
	stub, err := m.GetDb(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil, nil
	}
	members, err := stub.GetSyncgroupMembers(ctx, call, toV23Id(sgId))
	if err != nil {
		return toMojoError(err), nil, nil
	}
	mojoMembers := make(map[string]mojom.SyncgroupMemberInfo, len(members))
	for name, member := range members {
		mojoMembers[name] = toMojoSyncgroupMemberInfo(member)
	}
	return toMojoError(err), mojoMembers, nil
}

////////////////////////////////////////
// Collection

func (m *mojoImpl) CollectionCreate(name, batchHandle string, mPerms mojom.Perms) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "Create"))
	stub, err := m.GetCollection(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	perms, err := toV23Perms(mPerms)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.Create(ctx, call, batchHandle, perms)
	return toMojoError(err), nil
}

func (m *mojoImpl) CollectionDestroy(name, batchHandle string) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "Destroy"))
	stub, err := m.GetCollection(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.Destroy(ctx, call, batchHandle)
	return toMojoError(err), nil
}

func (m *mojoImpl) CollectionExists(name, batchHandle string) (mojom.Error, bool, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "Exists"))
	stub, err := m.GetCollection(ctx, call, name)
	if err != nil {
		return toMojoError(err), false, nil
	}
	exists, err := stub.Exists(ctx, call, batchHandle)
	return toMojoError(err), exists, nil
}

func (m *mojoImpl) CollectionGetPermissions(name, batchHandle string) (mojom.Error, mojom.Perms, error) {
	return toMojomError(verror.NewErrNotImplemented(nil)), mojom.Perms{}, nil
}

func (m *mojoImpl) CollectionSetPermissions(name, batchHandle string, mPerms mojom.Perms) (mojom.Error, error) {
	return toMojomError(verror.NewErrNotImplemented(nil)), nil
}

func (m *mojoImpl) CollectionDeleteRange(name, batchHandle string, start, limit []byte) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "DeleteRange"))
	stub, err := m.GetCollection(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.DeleteRange(ctx, call, batchHandle, start, limit)
	return toMojoError(err), nil
}

type scanStreamImpl struct {
	ctx   *context.T
	proxy *mojom.ScanStream_Proxy
}

func (s *scanStreamImpl) Send(item interface{}) error {
	kv, ok := item.(wire.KeyValue)
	if !ok {
		return verror.NewErrInternal(s.ctx)
	}
	var value []byte
	if err := vom.Decode(kv.Value, &value); err != nil {
		return err
	}
	// proxy.OnKeyValue() blocks until the client acks the previous invocation,
	// thus providing flow control.
	return s.proxy.OnKeyValue(mojom.KeyValue{
		Key:   kv.Key,
		Value: value,
	})
}

func (s *scanStreamImpl) Recv(_ interface{}) error {
	// This should never be called.
	return verror.NewErrInternal(s.ctx)
}

var _ rpc.Stream = (*scanStreamImpl)(nil)

// TODO(nlacasse): Provide some way for the client to cancel the stream.
func (m *mojoImpl) CollectionScan(name, batchHandle string, start, limit []byte, ptr mojom.ScanStream_Pointer) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "Scan"))
	stub, err := m.GetCollection(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}

	proxy := mojom.NewScanStreamProxy(ptr, bindings.GetAsyncWaiter())

	collectionScanServerCallStub := &wire.CollectionScanServerCallStub{struct {
		rpc.Stream
		rpc.ServerCall
	}{
		&scanStreamImpl{ctx: ctx, proxy: proxy},
		call,
	}}

	go func() {
		var err = stub.Scan(ctx, collectionScanServerCallStub, batchHandle, start, limit)

		// NOTE(nlacasse): Since we are already streaming, we send any error back to
		// the client on the stream. The CollectionScan function itself should not
		// return an error at this point.
		proxy.OnDone(toMojoError(err))
	}()

	return mojom.Error{}, nil
}

////////////////////////////////////////
// Row

func (m *mojoImpl) RowExists(name, batchHandle string) (mojom.Error, bool, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.RowDesc, "Exists"))
	stub, err := m.GetRow(ctx, call, name)
	if err != nil {
		return toMojoError(err), false, nil
	}
	exists, err := stub.Exists(ctx, call, batchHandle)
	return toMojoError(err), exists, nil
}

func (m *mojoImpl) RowGet(name, batchHandle string) (mojom.Error, []byte, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.RowDesc, "Get"))
	stub, err := m.GetRow(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil, nil
	}
	vomBytes, err := stub.Get(ctx, call, batchHandle)

	var value []byte
	if err := vom.Decode(vomBytes, &value); err != nil {
		return toMojoError(err), nil, nil
	}
	return toMojoError(err), value, nil
}

func (m *mojoImpl) RowPut(name, batchHandle string, value []byte) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.RowDesc, "Put"))
	stub, err := m.GetRow(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	// TODO(aghassemi): For now, Dart always gives us []byte, and here we convert
	// []byte to VOM-encoded []byte so that all values stored in Syncbase are
	// VOM-encoded. This will need to change once we support VDL/VOM in Dart.
	// https://github.com/vanadium/issues/issues/766
	vomBytes, err := vom.Encode(value)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.Put(ctx, call, batchHandle, vomBytes)
	return toMojoError(err), nil
}

func (m *mojoImpl) RowDelete(name, batchHandle string) (mojom.Error, error) {
	ctx, call := m.NewCtxCall(name, bridge.MethodDesc(wire.RowDesc, "Delete"))
	stub, err := m.GetRow(ctx, call, name)
	if err != nil {
		return toMojoError(err), nil
	}
	err = stub.Delete(ctx, call, batchHandle)
	return toMojoError(err), nil
}
