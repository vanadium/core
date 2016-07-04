// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build cgo

// TODO(sadovsky): Make DbWatchPatterns and CollectionScan cancelable, e.g. by
// returning a cancel closure handle to the client.

// Syncbase C/Cgo API. Our strategy is to translate Cgo requests into Vanadium
// stub requests, and Vanadium stub responses into Cgo responses. As part of
// this procedure, we synthesize "fake" ctx and call objects to pass to the
// Vanadium stubs.
//
// Implementation notes:
// - This API partly mirrors the Syncbase RPC API. Many methods take 'cName' as
//   their first argument; this is a service-relative Vanadium object name. For
//   example, the 'cName' argument to DbCreate is an encoded database id.
// - All exported function and type names start with "v23_syncbase_", to avoid
//   colliding with desired client library names.
// - Exported functions take input arguments by value, optional input arguments
//   by pointer, and output arguments by pointer.
// - Caller transfers ownership of all input arguments to callee; callee
//   transfers ownership of all output arguments to caller. If a function
//   returns an error, other output arguments need not be freed.
// - Variables with Cgo-specific types have names that start with "c".

package main

import (
	"os"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/permissions"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/syncbase/bridge"
	"v.io/x/ref/services/syncbase/bridge/cgo/refmap"
	"v.io/x/ref/services/syncbase/discovery"
	"v.io/x/ref/services/syncbase/syncbaselib"
	"v.io/x/ref/test/testutil"
)

/*
#include "lib.h"

static void CallDbWatchPatternsCallbacksOnChange(v23_syncbase_DbWatchPatternsCallbacks cbs, v23_syncbase_WatchChange wc) {
  cbs.onChange(cbs.handle, wc);
}
static void CallDbWatchPatternsCallbacksOnError(v23_syncbase_DbWatchPatternsCallbacks cbs, v23_syncbase_VError err) {
  cbs.onError(cbs.handle, err);
}

static void CallCollectionScanCallbacksOnKeyValue(v23_syncbase_CollectionScanCallbacks cbs, v23_syncbase_KeyValue kv) {
  cbs.onKeyValue(cbs.handle, kv);
}
static void CallCollectionScanCallbacksOnDone(v23_syncbase_CollectionScanCallbacks cbs, v23_syncbase_VError err) {
  cbs.onDone(cbs.handle, err);
}

static void CallDbSyncgroupInvitesOnInvite(v23_syncbase_DbSyncgroupInvitesCallbacks cbs, v23_syncbase_Invite i) {
  cbs.onInvite(cbs.handle, i);
}

static void CallNeighborhoodScanCallbacksOnPeer(v23_syncbase_NeighborhoodScanCallbacks cbs, v23_syncbase_AppPeer p) {
  cbs.onPeer(cbs.handle, p);
}
*/
import "C"

// Global state, initialized by v23_syncbase_Init.
var (
	b *bridge.Bridge
	// rootDir is the root directory for credentials and syncbase storage.
	rootDir string
	// clientUnderstandsVOM specifies whether the Cgo layer should assume
	// the client does VOM encoding and decoding. If false, the Cgo layer
	// itself does VOM encoding and decoding, and the client deals in byte
	// arrays.
	clientUnderstandsVOM bool
	// testLogin indicates if the test login mode is used. This is triggered
	// by using an empty identity provider.
	testLogin bool
	// neighborhoodAdStatus tracks the status of the neighborhood advertisement.
	neighborhoodAdStatus *adStatus
)

var globalRefMap = refmap.NewRefMap()

// TODO(razvanm): Replace the function arguments with an options struct.
//export v23_syncbase_Init
func v23_syncbase_Init(cClientUnderstandVom C.v23_syncbase_Bool, cRootDir C.v23_syncbase_String, cTestLogin C.v23_syncbase_Bool) {
	if b != nil {
		panic("v23_syncbase_Init called again before a v23_syncbase_Shutdown")
	}
	// Strip all flags beyond the binary name; otherwise, v23.Init will fail when it encounters
	// unknown flags passed by Xcode, e.g. NSTreatUnknownArgumentsAsOpen.
	os.Args = os.Args[:1]
	ctx, shutdown := v23.Init()
	b = &bridge.Bridge{Ctx: ctx, Shutdown: shutdown}
	rootDir = cRootDir.extract()
	clientUnderstandsVOM = cClientUnderstandVom.extract()
	testLogin = cTestLogin.extract()
	var cErr C.v23_syncbase_VError
	if isLoggedIn() {
		v23_syncbase_Serve(&cErr)
	}
	neighborhoodAdStatus = newAdStatus()
}

type adStatus struct {
	isAdvertising bool
	cancel        context.CancelFunc
	done          <-chan struct{}
}

func newAdStatus() *adStatus {
	return &adStatus{}
}

func (a *adStatus) store(cancel context.CancelFunc, done <-chan struct{}) {
	a.isAdvertising = true
	a.cancel = cancel
	a.done = done
}

func (a *adStatus) stop() {
	if a.isAdvertising {
		a.cancel()
		<-a.done
		a.isAdvertising = false
		a.cancel = nil
		a.done = nil
	}
}

//export v23_syncbase_Serve
func v23_syncbase_Serve(cErr *C.v23_syncbase_VError) {
	if !isLoggedIn() {
		cErr.init(verror.New(verror.ErrInternal, nil, "not logged in"))
	}
	srv, disp, cleanup := syncbaselib.Serve(b.Ctx, syncbaselib.Opts{
		// TODO(sadovsky): Make and pass a subdir of rootDir here, so
		// that rootDir can also be used for credentials persistence.
		RootDir: rootDir,
	})
	b.Srv = srv
	b.Disp = disp
	b.Cleanup = cleanup
}

//export v23_syncbase_Shutdown
func v23_syncbase_Shutdown() {
	if b == nil {
		return
	}

	if b.Cleanup != nil {
		b.Cleanup()
		b.Cleanup = nil
	}

	b.Shutdown()
	b = nil
	testLogin = false
}

////////////////////////////////////////
// OAuth

func isLoggedIn() bool {
	_, _, err := util.AppAndUserPatternFromBlessings(security.DefaultBlessingNames(v23.GetPrincipal(b.Ctx))...)
	return err == nil
}

//export v23_syncbase_IsLoggedIn
func v23_syncbase_IsLoggedIn(cIsLoggedIn *C.v23_syncbase_Bool) {
	cIsLoggedIn.init(isLoggedIn())
}

//export v23_syncbase_Login
func v23_syncbase_Login(cOAuthProvider C.v23_syncbase_String, cOAuthToken C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	if isLoggedIn() {
		return
	}

	if testLogin {
		ctx, err := v23.WithPrincipal(b.Ctx, testutil.NewPrincipal("root:o:app:user"))
		if err != nil {
			panic(err)
		}
		b.Ctx = ctx
	} else {
		err := bridge.SeekAndSetBlessings(b.Ctx, cOAuthProvider.extract(), cOAuthToken.extract())
		if err != nil {
			cErr.init(err)
			return
		}
	}
	v23_syncbase_Serve(cErr)
}

////////////////////////////////////////
// Glob utils

func listChildIds(name string, cIds *C.v23_syncbase_Ids, cErr *C.v23_syncbase_VError) {
	ctx, call := b.NewCtxCall(name, rpc.MethodDesc{
		Name: "GlobChildren__",
	})
	stub, err := b.GetGlobber(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	gcsCall := &globChildrenServerCall{call, ctx, make([]wire.Id, 0)}
	g, err := glob.Parse("*")
	if err != nil {
		cErr.init(err)
		return
	}
	if err := stub.GlobChildren__(ctx, gcsCall, g.Head()); err != nil {
		cErr.init(err)
		return
	}
	cIds.init(gcsCall.Ids)
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

//export v23_syncbase_ServiceGetPermissions
func v23_syncbase_ServiceGetPermissions(cPerms *C.v23_syncbase_Permissions, cVersion *C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	ctx, call := b.NewCtxCall("", bridge.MethodDesc(permissions.ObjectDesc, "GetPermissions"))
	stub, err := b.GetService(ctx, call)
	if err != nil {
		cErr.init(err)
		return
	}
	perms, version, err := stub.GetPermissions(ctx, call)
	if err != nil {
		cErr.init(err)
		return
	}
	cPerms.init(perms)
	cVersion.init(version)
}

//export v23_syncbase_ServiceSetPermissions
func v23_syncbase_ServiceSetPermissions(cPerms C.v23_syncbase_Permissions, cVersion C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	perms := cPerms.extract()
	version := cVersion.extract()
	ctx, call := b.NewCtxCall("", bridge.MethodDesc(permissions.ObjectDesc, "SetPermissions"))
	stub, err := b.GetService(ctx, call)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.SetPermissions(ctx, call, perms, version))
}

//export v23_syncbase_ServiceListDatabases
func v23_syncbase_ServiceListDatabases(cIds *C.v23_syncbase_Ids, cErr *C.v23_syncbase_VError) {
	// TODO(sadovsky): This is broken; it always returns an empty list.
	listChildIds("", cIds, cErr)
}

////////////////////////////////////////
// Service

//export v23_syncbase_NeighborhoodStartAdvertising
func v23_syncbase_NeighborhoodStartAdvertising(cVisibility C.v23_syncbase_Strings, cErr *C.v23_syncbase_VError) {
	// Cancel an old ad, if necessary.
	v23_syncbase_NeighborhoodStopAdvertising()

	// Initialize app advertisement.
	advCtx, cancel := context.WithCancel(b.Ctx)
	visibility := cVisibility.extract()
	visBlessingPatterns := make([]security.BlessingPattern, len(visibility))
	for i, vis := range visibility {
		visBlessingPatterns[i] = security.BlessingPattern(vis)
	}
	doneAd, err := discovery.AdvertiseApp(advCtx, visBlessingPatterns)
	if err != nil {
		cErr.init(err)
		return
	}

	// Remember the ad's cancellation information.
	neighborhoodAdStatus.store(cancel, doneAd)
}

//export v23_syncbase_NeighborhoodStopAdvertising
func v23_syncbase_NeighborhoodStopAdvertising() {
	neighborhoodAdStatus.stop()
}

//export v23_syncbase_NeighborhoodIsAdvertising
func v23_syncbase_NeighborhoodIsAdvertising(cBool *C.v23_syncbase_Bool) {
	cBool.init(neighborhoodAdStatus.isAdvertising)
}

//export v23_syncbase_NeighborhoodNewScan
func v23_syncbase_NeighborhoodNewScan(cbs C.v23_syncbase_NeighborhoodScanCallbacks, cUint64 *C.uint64_t, cErr *C.v23_syncbase_VError) {
	scanCtx, cancel := context.WithCancel(b.Ctx)
	scanChan := make(chan discovery.AppPeer)
	err := discovery.ListenForAppPeers(scanCtx, scanChan)
	if err != nil {
		cErr.init(err)
		return
	}

	// Forward the scan results to the callback.
	go func() {
		for {
			peer, ok := <-scanChan
			if !ok {
				break
			}
			cPeer := C.v23_syncbase_AppPeer{}
			cPeer.init(peer)
			C.CallNeighborhoodScanCallbacksOnPeer(cbs, cPeer)
		}
	}()

	// Remember the scan's cancellation information and return the scan's id.
	x := C.uint64_t(globalRefMap.Add(cancel))
	cUint64 = &x
}

//export v23_syncbase_NeighborhoodStopScan
func v23_syncbase_NeighborhoodStopScan(cUint64 C.uint64_t) {
	if cancel, ok := globalRefMap.Remove(uint64(cUint64)).(context.CancelFunc); ok && cancel != nil {
		cancel()
	}
}

////////////////////////////////////////
// Database

//export v23_syncbase_DbCreate
func v23_syncbase_DbCreate(cName C.v23_syncbase_String, cPerms C.v23_syncbase_Permissions, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	perms := cPerms.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Create"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.Create(ctx, call, nil, perms))
}

//export v23_syncbase_DbDestroy
func v23_syncbase_DbDestroy(cName C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Destroy"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.Destroy(ctx, call))
}

//export v23_syncbase_DbExists
func v23_syncbase_DbExists(cName C.v23_syncbase_String, cExists *C.v23_syncbase_Bool, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Exists"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	exists, err := stub.Exists(ctx, call)
	if err != nil {
		cErr.init(err)
		return
	}
	cExists.init(exists)
}

//export v23_syncbase_DbListCollections
func v23_syncbase_DbListCollections(cName, cBatchHandle C.v23_syncbase_String, cIds *C.v23_syncbase_Ids, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "ListCollections"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	ids, err := stub.ListCollections(ctx, call, batchHandle)
	if err != nil {
		cErr.init(err)
		return
	}
	cIds.init(ids)
}

//export v23_syncbase_DbBeginBatch
func v23_syncbase_DbBeginBatch(cName C.v23_syncbase_String, cOpts C.v23_syncbase_BatchOptions, cBatchHandle *C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	opts := cOpts.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "BeginBatch"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	batchHandle, err := stub.BeginBatch(ctx, call, opts)
	if err != nil {
		cErr.init(err)
		return
	}
	cBatchHandle.init(string(batchHandle))
}

//export v23_syncbase_DbCommit
func v23_syncbase_DbCommit(cName, cBatchHandle C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Commit"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.Commit(ctx, call, batchHandle))
}

//export v23_syncbase_DbAbort
func v23_syncbase_DbAbort(cName, cBatchHandle C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseDesc, "Abort"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.Abort(ctx, call, batchHandle))
}

//export v23_syncbase_DbGetPermissions
func v23_syncbase_DbGetPermissions(cName C.v23_syncbase_String, cPerms *C.v23_syncbase_Permissions, cVersion *C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(permissions.ObjectDesc, "GetPermissions"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	perms, version, err := stub.GetPermissions(ctx, call)
	if err != nil {
		cErr.init(err)
		return
	}
	cPerms.init(perms)
	cVersion.init(version)
}

//export v23_syncbase_DbSetPermissions
func v23_syncbase_DbSetPermissions(cName C.v23_syncbase_String, cPerms C.v23_syncbase_Permissions, cVersion C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	perms := cPerms.extract()
	version := cVersion.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(permissions.ObjectDesc, "SetPermissions"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.SetPermissions(ctx, call, perms, version))
}

//export v23_syncbase_DbGetResumeMarker
func v23_syncbase_DbGetResumeMarker(cName, cBatchHandle C.v23_syncbase_String, cMarker *C.v23_syncbase_Bytes, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseWatcherDesc, "GetResumeMarker"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	marker, err := stub.GetResumeMarker(ctx, call, batchHandle)
	if err != nil {
		cErr.init(err)
		return
	}
	cMarker.init(marker)
}

type watchStreamImpl struct {
	ctx *context.T
	cbs C.v23_syncbase_DbWatchPatternsCallbacks
}

func (s *watchStreamImpl) Send(item interface{}) error {
	wireWC, ok := item.(watch.Change)
	if !ok {
		return verror.NewErrInternal(s.ctx)
	}
	if wireWC.State == watch.InitialStateSkipped {
		return nil
	}
	// C.CallDbWatchPatternsCallbacksOnChange() blocks until the client acks the
	// previous invocation, thus providing flow control.
	cWatchChange := C.v23_syncbase_WatchChange{}
	cWatchChange.init(*syncbase.ToWatchChange(wireWC))
	C.CallDbWatchPatternsCallbacksOnChange(s.cbs, cWatchChange)
	return nil
}

func (s *watchStreamImpl) Recv(_ interface{}) error {
	// This should never be called.
	return verror.NewErrInternal(s.ctx)
}

var _ rpc.Stream = (*watchStreamImpl)(nil)

//export v23_syncbase_DbWatchPatterns
func v23_syncbase_DbWatchPatterns(cName C.v23_syncbase_String, cResumeMarker C.v23_syncbase_Bytes, cPatterns C.v23_syncbase_CollectionRowPatterns, cbs C.v23_syncbase_DbWatchPatternsCallbacks, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	resumeMarker := watch.ResumeMarker(cResumeMarker.extract())
	patterns := cPatterns.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.DatabaseWatcherDesc, "WatchPatterns"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}

	streamStub := &wire.DatabaseWatcherWatchPatternsServerCallStub{struct {
		rpc.Stream
		rpc.ServerCall
	}{
		&watchStreamImpl{ctx: ctx, cbs: cbs},
		call,
	}}

	go func() {
		err := stub.WatchPatterns(ctx, streamStub, resumeMarker, patterns)
		// Note: Since we are now streaming, any new error must be sent back on the
		// stream; the function itself should not return an error at this point.
		cErr := C.v23_syncbase_VError{}
		cErr.init(err)
		C.CallDbWatchPatternsCallbacksOnError(cbs, cErr)
	}()
}

////////////////////////////////////////
// SyncgroupManager

//export v23_syncbase_DbListSyncgroups
func v23_syncbase_DbListSyncgroups(cName C.v23_syncbase_String, cIds *C.v23_syncbase_Ids, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "ListSyncgroups"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	ids, err := stub.ListSyncgroups(ctx, call)
	if err != nil {
		cErr.init(err)
		return
	}
	cIds.init(ids)
}

//export v23_syncbase_DbCreateSyncgroup
func v23_syncbase_DbCreateSyncgroup(cName C.v23_syncbase_String, cSgId C.v23_syncbase_Id, cSpec C.v23_syncbase_SyncgroupSpec, cMyInfo C.v23_syncbase_SyncgroupMemberInfo, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	sgId := cSgId.toId()
	spec := cSpec.toSyncgroupSpec()
	myInfo := cMyInfo.toSyncgroupMemberInfo()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "CreateSyncgroup"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.CreateSyncgroup(ctx, call, sgId, spec, myInfo))
}

//export v23_syncbase_DbJoinSyncgroup
func v23_syncbase_DbJoinSyncgroup(cName, cRemoteSyncbaseName C.v23_syncbase_String, cExpectedSyncbaseBlessings C.v23_syncbase_Strings, cSgId C.v23_syncbase_Id, cMyInfo C.v23_syncbase_SyncgroupMemberInfo, cSpec *C.v23_syncbase_SyncgroupSpec, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	remoteSyncbaseName := cRemoteSyncbaseName.extract()
	expectedSyncbaseBlessings := cExpectedSyncbaseBlessings.extract()
	sgId := cSgId.toId()
	myInfo := cMyInfo.toSyncgroupMemberInfo()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "JoinSyncgroup"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	spec, err := stub.JoinSyncgroup(ctx, call, remoteSyncbaseName, expectedSyncbaseBlessings, sgId, myInfo)
	if err != nil {
		cErr.init(err)
		return
	}
	cSpec.init(spec)
}

//export v23_syncbase_DbLeaveSyncgroup
func v23_syncbase_DbLeaveSyncgroup(cName C.v23_syncbase_String, cSgId C.v23_syncbase_Id, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	sgId := cSgId.toId()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "LeaveSyncgroup"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.LeaveSyncgroup(ctx, call, sgId))
}

//export v23_syncbase_DbDestroySyncgroup
func v23_syncbase_DbDestroySyncgroup(cName C.v23_syncbase_String, cSgId C.v23_syncbase_Id, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	sgId := cSgId.toId()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "DestroySyncgroup"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.DestroySyncgroup(ctx, call, sgId))
}

//export v23_syncbase_DbEjectFromSyncgroup
func v23_syncbase_DbEjectFromSyncgroup(cName C.v23_syncbase_String, cSgId C.v23_syncbase_Id, cMember C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	sgId := cSgId.toId()
	member := cMember.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "EjectFromSyncgroup"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.EjectFromSyncgroup(ctx, call, sgId, member))
}

//export v23_syncbase_DbGetSyncgroupSpec
func v23_syncbase_DbGetSyncgroupSpec(cName C.v23_syncbase_String, cSgId C.v23_syncbase_Id, cSpec *C.v23_syncbase_SyncgroupSpec, cVersion *C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	sgId := cSgId.toId()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "GetSyncgroupSpec"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	spec, version, err := stub.GetSyncgroupSpec(ctx, call, sgId)
	if err != nil {
		cErr.init(err)
		return
	}
	cSpec.init(spec)
	cVersion.init(version)
}

//export v23_syncbase_DbSetSyncgroupSpec
func v23_syncbase_DbSetSyncgroupSpec(cName C.v23_syncbase_String, cSgId C.v23_syncbase_Id, cSpec C.v23_syncbase_SyncgroupSpec, cVersion C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	sgId := cSgId.toId()
	spec := cSpec.toSyncgroupSpec()
	version := cVersion.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "SetSyncgroupSpec"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.SetSyncgroupSpec(ctx, call, sgId, spec, version))
}

//export v23_syncbase_DbGetSyncgroupMembers
func v23_syncbase_DbGetSyncgroupMembers(cName C.v23_syncbase_String, cSgId C.v23_syncbase_Id, cMembers *C.v23_syncbase_SyncgroupMemberInfoMap, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	sgId := cSgId.toId()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.SyncgroupManagerDesc, "GetSyncgroupMembers"))
	stub, err := b.GetDb(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	members, err := stub.GetSyncgroupMembers(ctx, call, sgId)
	if err != nil {
		cErr.init(err)
		return
	}
	cMembers.init(members)
}

////////////////////////////////////////
// Syncgroup Invites

//export v23_syncbase_DbSyncgroupInvitesNewScan
func v23_syncbase_DbSyncgroupInvitesNewScan(cName C.v23_syncbase_String, cbs C.v23_syncbase_DbSyncgroupInvitesCallbacks, cUint64 *C.uint64_t, cErr *C.v23_syncbase_VError) {
	encodedId := cName.extract()
	dbId, err := util.DecodeId(encodedId)
	if err != nil {
		cErr.init(err)
		return
	}

	scanCtx, cancel := context.WithCancel(b.Ctx)
	scanChan := make(chan discovery.Invite)
	err = discovery.ListenForInvites(scanCtx, dbId, scanChan)
	if err != nil {
		cErr.init(err)
		return
	}

	// Forward the scan results to the callback.
	go func() {
		for {
			invite, ok := <-scanChan
			if !ok {
				break
			}
			cInvite := C.v23_syncbase_Invite{}
			cInvite.init(invite)
			C.CallDbSyncgroupInvitesOnInvite(cbs, cInvite)
		}
	}()

	// Remember the scan's cancellation information and return the scan's id.
	x := C.uint64_t(globalRefMap.Add(cancel))
	cUint64 = &x
}

//export v23_syncbase_DbSyncgroupInvitesStopScan
func v23_syncbase_DbSyncgroupInvitesStopScan(cUint64 C.uint64_t) {
	if cancel, ok := globalRefMap.Remove(uint64(cUint64)).(context.CancelFunc); ok && cancel != nil {
		cancel()
	}
}

////////////////////////////////////////
// Collection

//export v23_syncbase_CollectionCreate
func v23_syncbase_CollectionCreate(cName, cBatchHandle C.v23_syncbase_String, cPerms C.v23_syncbase_Permissions, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	perms := cPerms.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "Create"))
	stub, err := b.GetCollection(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.Create(ctx, call, batchHandle, perms))
}

//export v23_syncbase_CollectionDestroy
func v23_syncbase_CollectionDestroy(cName, cBatchHandle C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "Destroy"))
	stub, err := b.GetCollection(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.Destroy(ctx, call, batchHandle))
}

//export v23_syncbase_CollectionExists
func v23_syncbase_CollectionExists(cName, cBatchHandle C.v23_syncbase_String, cExists *C.v23_syncbase_Bool, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "Exists"))
	stub, err := b.GetCollection(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	exists, err := stub.Exists(ctx, call, batchHandle)
	if err != nil {
		cErr.init(err)
		return
	}
	cExists.init(exists)
}

//export v23_syncbase_CollectionGetPermissions
func v23_syncbase_CollectionGetPermissions(cName, cBatchHandle C.v23_syncbase_String, cPerms *C.v23_syncbase_Permissions, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "GetPermissions"))
	stub, err := b.GetCollection(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	perms, err := stub.GetPermissions(ctx, call, batchHandle)
	if err != nil {
		cErr.init(err)
		return
	}
	cPerms.init(perms)
}

//export v23_syncbase_CollectionSetPermissions
func v23_syncbase_CollectionSetPermissions(cName, cBatchHandle C.v23_syncbase_String, cPerms C.v23_syncbase_Permissions, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	perms := cPerms.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "SetPermissions"))
	stub, err := b.GetCollection(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.SetPermissions(ctx, call, batchHandle, perms))
}

//export v23_syncbase_CollectionDeleteRange
func v23_syncbase_CollectionDeleteRange(cName, cBatchHandle C.v23_syncbase_String, cStart, cLimit C.v23_syncbase_Bytes, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	start, limit := cStart.extract(), cLimit.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "DeleteRange"))
	stub, err := b.GetCollection(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.DeleteRange(ctx, call, batchHandle, start, limit))
}

type scanStreamImpl struct {
	ctx *context.T
	cbs C.v23_syncbase_CollectionScanCallbacks
}

func (s *scanStreamImpl) Send(item interface{}) error {
	kv, ok := item.(wire.KeyValue)
	if !ok {
		return verror.NewErrInternal(s.ctx)
	}
	var value []byte
	var err error
	if clientUnderstandsVOM {
		value, err = vom.Encode(kv.Value)
	} else {
		rawBytes := (*vom.RawBytes)(kv.Value)
		err = rawBytes.ToValue(&value)
	}
	if err != nil {
		return err
	}
	// C.CallCollectionScanCallbacksOnKeyValue() blocks until the client acks the
	// previous invocation, thus providing flow control.
	cKeyValue := C.v23_syncbase_KeyValue{}
	cKeyValue.init(kv.Key, value)
	C.CallCollectionScanCallbacksOnKeyValue(s.cbs, cKeyValue)
	return nil
}

func (s *scanStreamImpl) Recv(_ interface{}) error {
	// This should never be called.
	return verror.NewErrInternal(s.ctx)
}

var _ rpc.Stream = (*scanStreamImpl)(nil)

//export v23_syncbase_CollectionScan
func v23_syncbase_CollectionScan(cName, cBatchHandle C.v23_syncbase_String, cStart, cLimit C.v23_syncbase_Bytes, cbs C.v23_syncbase_CollectionScanCallbacks, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	start, limit := cStart.extract(), cLimit.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.CollectionDesc, "Scan"))
	stub, err := b.GetCollection(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}

	streamStub := &wire.CollectionScanServerCallStub{struct {
		rpc.Stream
		rpc.ServerCall
	}{
		&scanStreamImpl{ctx: ctx, cbs: cbs},
		call,
	}}

	go func() {
		err := stub.Scan(ctx, streamStub, batchHandle, start, limit)
		// Note: Since we are now streaming, any new error must be sent back on the
		// stream; the function itself should not return an error at this point.
		cErr := C.v23_syncbase_VError{}
		cErr.init(err)
		C.CallCollectionScanCallbacksOnDone(cbs, cErr)
	}()
}

////////////////////////////////////////
// Row

//export v23_syncbase_RowExists
func v23_syncbase_RowExists(cName, cBatchHandle C.v23_syncbase_String, cExists *C.v23_syncbase_Bool, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.RowDesc, "Exists"))
	stub, err := b.GetRow(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	exists, err := stub.Exists(ctx, call, batchHandle)
	if err != nil {
		cErr.init(err)
		return
	}
	cExists.init(exists)
}

//export v23_syncbase_RowGet
func v23_syncbase_RowGet(cName, cBatchHandle C.v23_syncbase_String, cValue *C.v23_syncbase_Bytes, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.RowDesc, "Get"))
	stub, err := b.GetRow(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	valueAsRawBytes, err := stub.Get(ctx, call, batchHandle)
	if err != nil {
		cErr.init(err)
		return
	}
	var value []byte
	if clientUnderstandsVOM {
		value, err = vom.Encode(valueAsRawBytes)
	} else {
		err = valueAsRawBytes.ToValue(&value)
	}
	if err != nil {
		cErr.init(err)
		return
	}
	cValue.init(value)
}

//export v23_syncbase_RowPut
func v23_syncbase_RowPut(cName, cBatchHandle C.v23_syncbase_String, cValue C.v23_syncbase_Bytes, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	value := cValue.extract()
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.RowDesc, "Put"))
	stub, err := b.GetRow(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	var valueAsRawBytes *vom.RawBytes
	if clientUnderstandsVOM {
		var bytes vom.RawBytes
		err = vom.Decode(value, &bytes)
		if err == nil {
			valueAsRawBytes = &bytes
		}
	} else {
		valueAsRawBytes, err = vom.RawBytesFromValue(value)
	}
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.Put(ctx, call, batchHandle, valueAsRawBytes))
}

//export v23_syncbase_RowDelete
func v23_syncbase_RowDelete(cName, cBatchHandle C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	name := cName.extract()
	batchHandle := wire.BatchHandle(cBatchHandle.extract())
	ctx, call := b.NewCtxCall(name, bridge.MethodDesc(wire.RowDesc, "Delete"))
	stub, err := b.GetRow(ctx, call, name)
	if err != nil {
		cErr.init(err)
		return
	}
	cErr.init(stub.Delete(ctx, call, batchHandle))
}

////////////////////////////////////////
// Misc utilities

//export v23_syncbase_Encode
func v23_syncbase_Encode(cName C.v23_syncbase_String, cEncoded *C.v23_syncbase_String) {
	cEncoded.init(util.Encode(cName.extract()))
}

//export v23_syncbase_EncodeId
func v23_syncbase_EncodeId(cId C.v23_syncbase_Id, cEncoded *C.v23_syncbase_String) {
	cEncoded.init(util.EncodeId(cId.toId()))
}

//export v23_syncbase_NamingJoin
func v23_syncbase_NamingJoin(cElements C.v23_syncbase_Strings, cJoined *C.v23_syncbase_String) {
	cJoined.init(naming.Join(cElements.extract()...))
}

////////////////////////////////////////
// Blessings

//export v23_syncbase_BlessingStoreDebugString
func v23_syncbase_BlessingStoreDebugString(cDebugString *C.v23_syncbase_String) {
	cDebugString.init(v23.GetPrincipal(b.Ctx).BlessingStore().DebugString())
}

//export v23_syncbase_AppBlessingFromContext
func v23_syncbase_AppBlessingFromContext(cAppBlessing *C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	// TODO(ivanpi): Rename to match Go.
	ab, _, err := util.AppAndUserPatternFromBlessings(security.DefaultBlessingNames(v23.GetPrincipal(b.Ctx))...)
	if err != nil {
		cErr.init(err)
		return
	}
	cAppBlessing.init(string(ab))
}

//export v23_syncbase_UserBlessingFromContext
func v23_syncbase_UserBlessingFromContext(cUserBlessing *C.v23_syncbase_String, cErr *C.v23_syncbase_VError) {
	// TODO(ivanpi): Rename to match Go.
	_, ub, err := util.AppAndUserPatternFromBlessings(security.DefaultBlessingNames(v23.GetPrincipal(b.Ctx))...)
	if err != nil {
		cErr.init(err)
		return
	}
	cUserBlessing.init(string(ub))
}
