// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android
// +build cgo

package main

import (
	"fmt"
	"unsafe"
)

/*
#include <stdlib.h>
#include <string.h>
#include "jni_wrapper.h"
#include "lib.h"

static jvalue* allocJValueArray(int elements) {
  return malloc(sizeof(jvalue) * elements);
}

static void setJValueArrayElement(jvalue* arr, int index, jvalue val) {
  arr[index] = val;
}

void v23_syncbase_internal_onChange(v23_syncbase_Handle handle, v23_syncbase_WatchChange);
void v23_syncbase_internal_onError(v23_syncbase_Handle handle, v23_syncbase_VError);

static v23_syncbase_DbWatchPatternsCallbacks newVWatchPatternsCallbacks() {
  v23_syncbase_DbWatchPatternsCallbacks cbs = {
  	0, v23_syncbase_internal_onChange, v23_syncbase_internal_onError};
  return cbs;
}

void v23_syncbase_internal_onKeyValue(v23_syncbase_Handle handle, v23_syncbase_KeyValue);
void v23_syncbase_internal_onDone(v23_syncbase_Handle handle, v23_syncbase_VError);

static v23_syncbase_CollectionScanCallbacks newVScanCallbacks() {
  v23_syncbase_CollectionScanCallbacks cbs = {
  	0, v23_syncbase_internal_onKeyValue, v23_syncbase_internal_onDone};
  return cbs;
}

void v23_syncbase_internal_onInvite(v23_syncbase_Handle handle, v23_syncbase_Invite);

static v23_syncbase_DbSyncgroupInvitesCallbacks newVSyncgroupInvitesCallbacks() {
  v23_syncbase_DbSyncgroupInvitesCallbacks cbs = {
  	0, v23_syncbase_internal_onInvite};
  return cbs;
}

void v23_syncbase_internal_onPeer(v23_syncbase_Handle handle, v23_syncbase_AppPeer);

static v23_syncbase_NeighborhoodScanCallbacks newVNeighborhoodScanCallbacks() {
  v23_syncbase_NeighborhoodScanCallbacks cbs = {
  	0, v23_syncbase_internal_onPeer};
  return cbs;
}
*/
import "C"

var (
	jVM                         *C.JavaVM
	arrayListClass              jArrayListClass
	changeTypeClass             jChangeType
	collectionRowPatternClass   jCollectionRowPattern
	entityTypeClass             jEntityType
	hashMapClass                jHashMap
	idClass                     jIdClass
	keyValueClass               jKeyValue
	neighborhoodPeerClass       jNeighborhoodPeer
	permissionsClass            jPermissions
	syncgroupInviteClass        jSyncgroupInvite
	syncgroupMemberInfoClass    jSyncgroupMemberInfo
	syncgroupSpecClass          jSyncgroupSpec
	verrorClass                 jVErrorClass
	versionedPermissionsClass   jVersionedPermissions
	versionedSyncgroupSpecClass jVersionedSyncgroupSpec
	watchChangeClass            jWatchChange
)

// JNI_OnLoad is called when System.loadLibrary is called. We need to cache the
// *JavaVM because that's the only way to get hold of a JNIEnv that is needed
// for any JNI operation.
//
// Reference: https://developer.android.com/training/articles/perf-jni.html#native_libraries
//
//export JNI_OnLoad
func JNI_OnLoad(vm *C.JavaVM, reserved unsafe.Pointer) C.jint {
	var env *C.JNIEnv
	if C.GetEnv(vm, unsafe.Pointer(&env), C.JNI_VERSION_1_6) != C.JNI_OK {
		return C.JNI_ERR
	}
	jVM = vm

	arrayListClass = newJArrayListClass(env)
	changeTypeClass = newJChangeType(env)
	collectionRowPatternClass = newJCollectionRowPattern(env)
	entityTypeClass = newJEntityType(env)
	hashMapClass = newJHashMap(env)
	idClass = newJIdClass(env)
	keyValueClass = newJKeyValue(env)
	neighborhoodPeerClass = newJNeighborhoodPeer(env)
	permissionsClass = newJPermissions(env)
	syncgroupInviteClass = newJSyncgroupInvite(env)
	syncgroupMemberInfoClass = newJSyncgroupMemberInfo(env)
	syncgroupSpecClass = newJSyncgroupSpec(env)
	verrorClass = newJVErrorClass(env)
	versionedPermissionsClass = newJVersionedPermissions(env)
	versionedSyncgroupSpecClass = newJVersionedSyncgroupSpec(env)
	watchChangeClass = newJWatchChange(env)

	return C.JNI_VERSION_1_6
}

//export Java_io_v_syncbase_internal_Service_Init
func Java_io_v_syncbase_internal_Service_Init(env *C.JNIEnv, cls C.jclass, initRoot C.jstring, testLogin C.jboolean) {
	cInitRoot := newVStringFromJava(env, initRoot)
	v23_syncbase_Init(C.v23_syncbase_Bool(1), cInitRoot, C.v23_syncbase_Bool(testLogin))
}

//export Java_io_v_syncbase_internal_Service_Serve
func Java_io_v_syncbase_internal_Service_Serve(env *C.JNIEnv, cls C.jclass) {
	var cErr C.v23_syncbase_VError
	v23_syncbase_Serve(&cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Service_Shutdown
func Java_io_v_syncbase_internal_Service_Shutdown(env *C.JNIEnv, cls C.jclass) {
	v23_syncbase_Shutdown()
}

//export Java_io_v_syncbase_internal_Service_Login
func Java_io_v_syncbase_internal_Service_Login(env *C.JNIEnv, cls C.jclass, provider C.jstring, token C.jstring) {
	cProvider := newVStringFromJava(env, provider)
	cToken := newVStringFromJava(env, token)
	var cErr C.v23_syncbase_VError
	v23_syncbase_Login(cProvider, cToken, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Service_IsLoggedIn
func Java_io_v_syncbase_internal_Service_IsLoggedIn(env *C.JNIEnv, cls C.jclass) C.jboolean {
	var r C.v23_syncbase_Bool
	v23_syncbase_IsLoggedIn(&r)
	return C.jboolean(r)
}

//export Java_io_v_syncbase_internal_Service_GetPermissions
func Java_io_v_syncbase_internal_Service_GetPermissions(env *C.JNIEnv, cls C.jclass) C.jobject {
	var cPerms C.v23_syncbase_Permissions
	var cVersion C.v23_syncbase_String
	var cErr C.v23_syncbase_VError
	v23_syncbase_ServiceGetPermissions(&cPerms, &cVersion, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return newVersionedPermissions(env, &cPerms, &cVersion)
}

//export Java_io_v_syncbase_internal_Service_SetPermissions
func Java_io_v_syncbase_internal_Service_SetPermissions(env *C.JNIEnv, cls C.jclass, obj C.jobject) {
	cPerms, cVersion := newVPermissionsAndVersionFromJava(env, obj)
	var cErr C.v23_syncbase_VError
	v23_syncbase_ServiceSetPermissions(cPerms, cVersion, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Service_ListDatabases
func Java_io_v_syncbase_internal_Service_ListDatabases(env *C.JNIEnv, cls C.jclass) C.jobject {
	var cIds C.v23_syncbase_Ids
	var cErr C.v23_syncbase_VError
	v23_syncbase_ServiceListDatabases(&cIds, &cErr)
	obj := C.NewObjectA(env, arrayListClass.class, arrayListClass.init, nil)
	// TODO(razvanm): populate the obj based on the data from cIds.
	return obj
}

//export Java_io_v_syncbase_internal_Database_GetPermissions
func Java_io_v_syncbase_internal_Database_GetPermissions(env *C.JNIEnv, cls C.jclass, name C.jstring) C.jobject {
	cName := newVStringFromJava(env, name)
	var cPerms C.v23_syncbase_Permissions
	var cVersion C.v23_syncbase_String
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbGetPermissions(cName, &cPerms, &cVersion, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return newVersionedPermissions(env, &cPerms, &cVersion)
}

//export Java_io_v_syncbase_internal_Database_SetPermissions
func Java_io_v_syncbase_internal_Database_SetPermissions(env *C.JNIEnv, cls C.jclass, name C.jstring, obj C.jobject) {
	cName := newVStringFromJava(env, name)
	cPerms, cVersion := newVPermissionsAndVersionFromJava(env, obj)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbSetPermissions(cName, cPerms, cVersion, &cErr)
	maybeThrowException(env, &cErr)
}

// maybeThrowException takes ownership of cErr and throws a Java exception if
// cErr represents a non-nil error. Returns a boolean indicating whether an
// exception was thrown.
func maybeThrowException(env *C.JNIEnv, cErr *C.v23_syncbase_VError) bool {
	if obj := cErr.extractToJava(env); obj != nil {
		C.Throw(env, obj)
		return true
	}
	return false
}

//export Java_io_v_syncbase_internal_Database_Create
func Java_io_v_syncbase_internal_Database_Create(env *C.JNIEnv, cls C.jclass, name C.jstring, perms C.jobject) {
	cName := newVStringFromJava(env, name)
	cPerms := newVPermissionsFromJava(env, perms)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbCreate(cName, cPerms, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Database_Destroy
func Java_io_v_syncbase_internal_Database_Destroy(env *C.JNIEnv, cls C.jclass, name C.jstring) {
	cName := newVStringFromJava(env, name)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbDestroy(cName, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Database_Exists
func Java_io_v_syncbase_internal_Database_Exists(env *C.JNIEnv, cls C.jclass, name C.jstring) C.jboolean {
	cName := newVStringFromJava(env, name)
	var r C.v23_syncbase_Bool
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbExists(cName, &r, &cErr)
	maybeThrowException(env, &cErr)
	return C.jboolean(r)
}

//export Java_io_v_syncbase_internal_Database_BeginBatch
func Java_io_v_syncbase_internal_Database_BeginBatch(env *C.JNIEnv, cls C.jclass, name C.jstring, opts C.jobject) C.jstring {
	cName := newVStringFromJava(env, name)
	var cHandle C.v23_syncbase_String
	var cErr C.v23_syncbase_VError
	// TODO(razvanm): construct a C.v23_syncbase_BatchOptions from opts.
	v23_syncbase_DbBeginBatch(cName, C.v23_syncbase_BatchOptions{}, &cHandle, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return cHandle.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Database_ListCollections
func Java_io_v_syncbase_internal_Database_ListCollections(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) C.jobject {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var cIds C.v23_syncbase_Ids
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbListCollections(cName, cHandle, &cIds, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return cIds.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Database_Commit
func Java_io_v_syncbase_internal_Database_Commit(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbCommit(cName, cHandle, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Database_Abort
func Java_io_v_syncbase_internal_Database_Abort(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbAbort(cName, cHandle, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Database_GetResumeMarker
func Java_io_v_syncbase_internal_Database_GetResumeMarker(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) C.jbyteArray {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var cMarker C.v23_syncbase_Bytes
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbGetResumeMarker(cName, cHandle, &cMarker, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return cMarker.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Database_ListSyncgroups
func Java_io_v_syncbase_internal_Database_ListSyncgroups(env *C.JNIEnv, cls C.jclass, name C.jstring) C.jobject {
	cName := newVStringFromJava(env, name)
	var cIds C.v23_syncbase_Ids
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbListSyncgroups(cName, &cIds, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return cIds.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Database_CreateSyncgroup
func Java_io_v_syncbase_internal_Database_CreateSyncgroup(env *C.JNIEnv, cls C.jclass, name C.jstring, sgId C.jobject, spec C.jobject, info C.jobject) {
	cName := newVStringFromJava(env, name)
	cSgId := newVIdFromJava(env, sgId)
	cSpec := newVSyngroupSpecFromJava(env, spec)
	cMyInfo := newVSyncgroupMemberInfoFromJava(env, info)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbCreateSyncgroup(cName, cSgId, cSpec, cMyInfo, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Database_JoinSyncgroup
func Java_io_v_syncbase_internal_Database_JoinSyncgroup(env *C.JNIEnv, cls C.jclass, name C.jstring, remoteSyncbaseName C.jstring, expectedSyncbaseBlessings C.jobject, sgId C.jobject, info C.jobject) C.jobject {
	cName := newVStringFromJava(env, name)
	cRemoteSyncbaseName := newVStringFromJava(env, remoteSyncbaseName)
	cExpectedSyncbaseBlessings := newVStringsFromJava(env, expectedSyncbaseBlessings)
	cSgId := newVIdFromJava(env, sgId)
	cMyInfo := newVSyncgroupMemberInfoFromJava(env, info)
	var cSpec C.v23_syncbase_SyncgroupSpec
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbJoinSyncgroup(cName, cRemoteSyncbaseName, cExpectedSyncbaseBlessings, cSgId, cMyInfo, &cSpec, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return cSpec.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Database_LeaveSyncgroup
func Java_io_v_syncbase_internal_Database_LeaveSyncgroup(env *C.JNIEnv, cls C.jclass, name C.jstring, sgId C.jobject) {
	cName := newVStringFromJava(env, name)
	cSgId := newVIdFromJava(env, sgId)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbLeaveSyncgroup(cName, cSgId, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Database_DestroySyncgroup
func Java_io_v_syncbase_internal_Database_DestroySyncgroup(env *C.JNIEnv, cls C.jclass, name C.jstring, sgId C.jobject) {
	cName := newVStringFromJava(env, name)
	cSgId := newVIdFromJava(env, sgId)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbDestroySyncgroup(cName, cSgId, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Database_EjectFromSyncgroup
func Java_io_v_syncbase_internal_Database_EjectFromSyncgroup(env *C.JNIEnv, cls C.jclass, name C.jstring, sgId C.jobject, member C.jstring) {
	cName := newVStringFromJava(env, name)
	cSgId := newVIdFromJava(env, sgId)
	cMember := newVStringFromJava(env, member)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbEjectFromSyncgroup(cName, cSgId, cMember, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Database_GetSyncgroupSpec
func Java_io_v_syncbase_internal_Database_GetSyncgroupSpec(env *C.JNIEnv, cls C.jclass, name C.jstring, sgId C.jobject) C.jobject {
	cName := newVStringFromJava(env, name)
	cSgId := newVIdFromJava(env, sgId)
	var cSpec C.v23_syncbase_SyncgroupSpec
	var cVersion C.v23_syncbase_String
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbGetSyncgroupSpec(cName, cSgId, &cSpec, &cVersion, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	obj := C.NewObjectA(env, versionedSyncgroupSpecClass.class, versionedSyncgroupSpecClass.init, nil)
	C.SetObjectField(env, obj, versionedSyncgroupSpecClass.version, cVersion.extractToJava(env))
	C.SetObjectField(env, obj, versionedSyncgroupSpecClass.syncgroupSpec, cSpec.extractToJava(env))
	return obj
}

//export Java_io_v_syncbase_internal_Database_SetSyncgroupSpec
func Java_io_v_syncbase_internal_Database_SetSyncgroupSpec(env *C.JNIEnv, cls C.jclass, name C.jstring, sgId C.jobject, versionedSpec C.jobject) {
	cName := newVStringFromJava(env, name)
	cSgId := newVIdFromJava(env, sgId)
	cSpec, cVersion := newVSyngroupSpecAndVersionFromJava(env, versionedSpec)
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbSetSyncgroupSpec(cName, cSgId, cSpec, cVersion, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Database_GetSyncgroupMembers
func Java_io_v_syncbase_internal_Database_GetSyncgroupMembers(env *C.JNIEnv, cls C.jclass, name C.jstring, sgId C.jobject) C.jobject {
	cName := newVStringFromJava(env, name)
	cSgId := newVIdFromJava(env, sgId)
	var cMembers C.v23_syncbase_SyncgroupMemberInfoMap
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbGetSyncgroupMembers(cName, cSgId, &cMembers, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return cMembers.extractToJava(env)
}

//export v23_syncbase_internal_onChange
func v23_syncbase_internal_onChange(handle C.v23_syncbase_Handle, change C.v23_syncbase_WatchChange) {
	id := uint64(uintptr(handle))
	h := globalRefMap.Get(id).(*watchPatternsCallbacksHandle)
	env, free := getEnv()
	obj := change.extractToJava(env)
	arg := *(*C.jvalue)(unsafe.Pointer(&obj))
	C.CallVoidMethodA(env, C.jobject(unsafe.Pointer(h.obj)), h.callbacks.onChange, &arg)
	if C.ExceptionOccurred(env) != nil {
		C.ExceptionDescribe(env)
		panic("java exception")
	}
	free()
}

//export v23_syncbase_internal_onError
func v23_syncbase_internal_onError(handle C.v23_syncbase_Handle, error C.v23_syncbase_VError) {
	id := uint64(uintptr(handle))
	h := globalRefMap.Remove(id).(*watchPatternsCallbacksHandle)
	env, free := getEnv()
	obj := error.extractToJava(env)
	arg := *(*C.jvalue)(unsafe.Pointer(&obj))
	C.CallVoidMethodA(env, C.jobject(unsafe.Pointer(h.obj)), h.callbacks.onError, &arg)
	if C.ExceptionOccurred(env) != nil {
		C.ExceptionDescribe(env)
		panic("java exception")
	}
	C.DeleteGlobalRef(env, unsafe.Pointer(h.obj))
	free()
}

type watchPatternsCallbacksHandle struct {
	obj       uintptr
	callbacks jWatchPatternsCallbacks
}

//export Java_io_v_syncbase_internal_Database_WatchPatterns
func Java_io_v_syncbase_internal_Database_WatchPatterns(env *C.JNIEnv, cls C.jclass, name C.jstring, resumeMaker C.jbyteArray, patterns C.jobject, callbacks C.jobject) {
	cName := newVStringFromJava(env, name)
	cResumeMarker := newVBytesFromJava(env, resumeMaker)
	cPatterns := newVCollectionRowPatternsFromJava(env, patterns)
	cbs := C.newVWatchPatternsCallbacks()
	cbs.handle = C.v23_syncbase_Handle(uintptr(globalRefMap.Add(&watchPatternsCallbacksHandle{
		obj:       uintptr(unsafe.Pointer(C.NewGlobalRef(env, callbacks))),
		callbacks: newJWatchPatternsCallbacks(env, callbacks),
	})))
	var cErr C.v23_syncbase_VError
	v23_syncbase_DbWatchPatterns(cName, cResumeMarker, cPatterns, cbs, &cErr)
	maybeThrowException(env, &cErr)
}

//export v23_syncbase_internal_onInvite
func v23_syncbase_internal_onInvite(handle C.v23_syncbase_Handle, invite C.v23_syncbase_Invite) {
	id := uint64(uintptr(handle))
	h := globalRefMap.Get(id).(*inviteCallbacksHandle)
	env, free := getEnv()
	obj := invite.extractToJava(env)
	arg := *(*C.jvalue)(unsafe.Pointer(&obj))
	C.CallVoidMethodA(env, C.jobject(unsafe.Pointer(h.obj)), h.callbacks.onInvite, &arg)
	if C.ExceptionOccurred(env) != nil {
		C.ExceptionDescribe(env)
		panic("java exception")
	}
	free()
}

type inviteCallbacksHandle struct {
	obj       uintptr
	callbacks jSyncgroupInvitesCallbacks
}

//export Java_io_v_syncbase_internal_Database_SyncgroupInvitesNewScan
func Java_io_v_syncbase_internal_Database_SyncgroupInvitesNewScan(env *C.JNIEnv, cls C.jclass, name C.jstring, callbacks C.jobject) C.jlong {
	cName := newVStringFromJava(env, name)
	cbs := C.newVSyncgroupInvitesCallbacks()
	cbs.handle = C.v23_syncbase_Handle(uintptr(globalRefMap.Add(&inviteCallbacksHandle{
		obj:       uintptr(unsafe.Pointer(C.NewGlobalRef(env, callbacks))),
		callbacks: newJSyncgroupInvitesCallbacks(env, callbacks),
	})))
	var cErr C.v23_syncbase_VError
	var scanId C.uint64_t
	v23_syncbase_DbSyncgroupInvitesNewScan(cName, cbs, &scanId, &cErr)
	maybeThrowException(env, &cErr)
	return C.jlong(scanId)
}

//export Java_io_v_syncbase_internal_Database_SyncgroupInvitesStopScan
func Java_io_v_syncbase_internal_Database_SyncgroupInvitesStopScan(env *C.JNIEnv, cls C.jclass, scanId C.jlong) {
	v23_syncbase_DbSyncgroupInvitesStopScan(C.uint64_t(scanId))
}

//export Java_io_v_syncbase_internal_Collection_GetPermissions
func Java_io_v_syncbase_internal_Collection_GetPermissions(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) C.jobject {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var cPerms C.v23_syncbase_Permissions
	var cErr C.v23_syncbase_VError
	v23_syncbase_CollectionGetPermissions(cName, cHandle, &cPerms, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return cPerms.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Collection_SetPermissions
func Java_io_v_syncbase_internal_Collection_SetPermissions(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring, perms C.jobject) {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	cPerms := newVPermissionsFromJava(env, perms)
	var cErr C.v23_syncbase_VError
	v23_syncbase_CollectionSetPermissions(cName, cHandle, cPerms, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Collection_Create
func Java_io_v_syncbase_internal_Collection_Create(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring, perms C.jobject) {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	cPerms := newVPermissionsFromJava(env, perms)
	var cErr C.v23_syncbase_VError
	v23_syncbase_CollectionCreate(cName, cHandle, cPerms, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Collection_Destroy
func Java_io_v_syncbase_internal_Collection_Destroy(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var cErr C.v23_syncbase_VError
	v23_syncbase_CollectionDestroy(cName, cHandle, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Collection_Exists
func Java_io_v_syncbase_internal_Collection_Exists(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) C.jboolean {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var r C.v23_syncbase_Bool
	var cErr C.v23_syncbase_VError
	v23_syncbase_CollectionExists(cName, cHandle, &r, &cErr)
	maybeThrowException(env, &cErr)
	return C.jboolean(r)
}

//export Java_io_v_syncbase_internal_Collection_DeleteRange
func Java_io_v_syncbase_internal_Collection_DeleteRange(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring, start C.jbyteArray, limit C.jbyteArray) {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	cStart := newVBytesFromJava(env, start)
	cLimit := newVBytesFromJava(env, limit)
	var cErr C.v23_syncbase_VError
	v23_syncbase_CollectionDeleteRange(cName, cHandle, cStart, cLimit, &cErr)
	maybeThrowException(env, &cErr)
}

//export v23_syncbase_internal_onKeyValue
func v23_syncbase_internal_onKeyValue(handle C.v23_syncbase_Handle, keyValue C.v23_syncbase_KeyValue) {
	id := uint64(uintptr(handle))
	h := globalRefMap.Get(id).(*scanCallbacksHandle)
	env, free := getEnv()
	obj := keyValue.extractToJava(env)
	arg := *(*C.jvalue)(unsafe.Pointer(&obj))
	C.CallVoidMethodA(env, C.jobject(unsafe.Pointer(h.obj)), h.callbacks.onKeyValue, &arg)
	if C.ExceptionOccurred(env) != nil {
		C.ExceptionDescribe(env)
		panic("java exception")
	}
	free()
}

//export v23_syncbase_internal_onDone
func v23_syncbase_internal_onDone(handle C.v23_syncbase_Handle, error C.v23_syncbase_VError) {
	id := uint64(uintptr(handle))
	h := globalRefMap.Get(id).(*scanCallbacksHandle)
	env, free := getEnv()
	obj := error.extractToJava(env)
	arg := *(*C.jvalue)(unsafe.Pointer(&obj))
	C.CallVoidMethodA(env, C.jobject(unsafe.Pointer(h.obj)), h.callbacks.onDone, &arg)
	if C.ExceptionOccurred(env) != nil {
		C.ExceptionDescribe(env)
		panic("java exception")
	}
	C.DeleteGlobalRef(env, unsafe.Pointer(h.obj))
	free()
	globalRefMap.Remove(id)
}

type scanCallbacksHandle struct {
	obj       uintptr
	callbacks jScanCallbacks
}

//export Java_io_v_syncbase_internal_Collection_Scan
func Java_io_v_syncbase_internal_Collection_Scan(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring, start C.jbyteArray, limit C.jbyteArray, callbacks C.jobject) {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	cStart := newVBytesFromJava(env, start)
	cLimit := newVBytesFromJava(env, limit)
	cbs := C.newVScanCallbacks()
	cbs.handle = C.v23_syncbase_Handle(uintptr(globalRefMap.Add(&scanCallbacksHandle{
		obj:       uintptr(unsafe.Pointer(C.NewGlobalRef(env, callbacks))),
		callbacks: newJScanCallbacks(env, callbacks),
	})))
	var cErr C.v23_syncbase_VError
	v23_syncbase_CollectionScan(cName, cHandle, cStart, cLimit, cbs, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Neighborhood_StartAdvertising
func Java_io_v_syncbase_internal_Neighborhood_StartAdvertising(env *C.JNIEnv, cls C.jclass, visibility C.jobject) {
	cVisibility := newVStringsFromJava(env, visibility)
	var cErr C.v23_syncbase_VError
	v23_syncbase_NeighborhoodStartAdvertising(cVisibility, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Neighborhood_StopAdvertising
func Java_io_v_syncbase_internal_Neighborhood_StopAdvertising(env *C.JNIEnv, cls C.jclass) {
	v23_syncbase_NeighborhoodStopAdvertising()
}

//export Java_io_v_syncbase_internal_Neighborhood_IsAdvertising
func Java_io_v_syncbase_internal_Neighborhood_IsAdvertising(env *C.JNIEnv, cls C.jclass) C.jboolean {
	var x C.v23_syncbase_Bool
	v23_syncbase_NeighborhoodIsAdvertising(&x)
	return C.jboolean(x)
}

//export v23_syncbase_internal_onPeer
func v23_syncbase_internal_onPeer(handle C.v23_syncbase_Handle, peer C.v23_syncbase_AppPeer) {
	id := uint64(uintptr(handle))
	h := globalRefMap.Get(id).(*peerCallbacksHandle)
	env, free := getEnv()
	obj := peer.extractToJava(env)
	arg := *(*C.jvalue)(unsafe.Pointer(&obj))
	C.CallVoidMethodA(env, C.jobject(unsafe.Pointer(h.obj)), h.callbacks.onPeer, &arg)
	if C.ExceptionOccurred(env) != nil {
		C.ExceptionDescribe(env)
		panic("java exception")
	}
	free()
}

type peerCallbacksHandle struct {
	obj       uintptr
	callbacks jNeighborhoodScanCallbacks
}

//export Java_io_v_syncbase_internal_Neighborhood_NewScan
func Java_io_v_syncbase_internal_Neighborhood_NewScan(env *C.JNIEnv, cls C.jclass, callbacks C.jobject) C.jlong {
	cbs := C.newVNeighborhoodScanCallbacks()
	cbs.handle = C.v23_syncbase_Handle(uintptr(globalRefMap.Add(&peerCallbacksHandle{
		obj:       uintptr(unsafe.Pointer(C.NewGlobalRef(env, callbacks))),
		callbacks: newJNeigbhorhoodScanCallbacks(env, callbacks),
	})))
	var cErr C.v23_syncbase_VError
	var scanId C.uint64_t
	v23_syncbase_NeighborhoodNewScan(cbs, &scanId, &cErr)
	maybeThrowException(env, &cErr)
	return C.jlong(scanId)
}

//export Java_io_v_syncbase_internal_Neighborhood_StopScan
func Java_io_v_syncbase_internal_Neighborhood_StopScan(env *C.JNIEnv, cls C.jclass, scanId C.jlong) {
	v23_syncbase_NeighborhoodStopScan(C.uint64_t(scanId))
}

//export Java_io_v_syncbase_internal_Row_Exists
func Java_io_v_syncbase_internal_Row_Exists(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) C.jboolean {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var r C.v23_syncbase_Bool
	var cErr C.v23_syncbase_VError
	v23_syncbase_RowExists(cName, cHandle, &r, &cErr)
	maybeThrowException(env, &cErr)
	return C.jboolean(r)
}

//export Java_io_v_syncbase_internal_Row_Get
func Java_io_v_syncbase_internal_Row_Get(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) C.jbyteArray {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var r C.v23_syncbase_Bytes
	var cErr C.v23_syncbase_VError
	v23_syncbase_RowGet(cName, cHandle, &r, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return r.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Row_Put
func Java_io_v_syncbase_internal_Row_Put(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring, value C.jbyteArray) {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var cErr C.v23_syncbase_VError
	cValue := newVBytesFromJava(env, value)
	v23_syncbase_RowPut(cName, cHandle, cValue, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Row_Delete
func Java_io_v_syncbase_internal_Row_Delete(env *C.JNIEnv, cls C.jclass, name C.jstring, handle C.jstring) {
	cName := newVStringFromJava(env, name)
	cHandle := newVStringFromJava(env, handle)
	var cErr C.v23_syncbase_VError
	v23_syncbase_RowDelete(cName, cHandle, &cErr)
	maybeThrowException(env, &cErr)
}

//export Java_io_v_syncbase_internal_Blessings_DebugString
func Java_io_v_syncbase_internal_Blessings_DebugString(env *C.JNIEnv, cls C.jclass) C.jstring {
	var cDebugString C.v23_syncbase_String
	v23_syncbase_BlessingStoreDebugString(&cDebugString)
	return cDebugString.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Blessings_AppBlessingFromContext
func Java_io_v_syncbase_internal_Blessings_AppBlessingFromContext(env *C.JNIEnv, cls C.jclass) C.jstring {
	var cBlessing C.v23_syncbase_String
	var cErr C.v23_syncbase_VError
	v23_syncbase_AppBlessingFromContext(&cBlessing, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return cBlessing.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Blessings_UserBlessingFromContext
func Java_io_v_syncbase_internal_Blessings_UserBlessingFromContext(env *C.JNIEnv, cls C.jclass) C.jstring {
	var cBlessing C.v23_syncbase_String
	var cErr C.v23_syncbase_VError
	v23_syncbase_UserBlessingFromContext(&cBlessing, &cErr)
	if maybeThrowException(env, &cErr) {
		return nil
	}
	return cBlessing.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Util_Encode
func Java_io_v_syncbase_internal_Util_Encode(env *C.JNIEnv, cls C.jclass, s C.jstring) C.jstring {
	cPlainStr := newVStringFromJava(env, s)
	var cEncodedStr C.v23_syncbase_String
	v23_syncbase_Encode(cPlainStr, &cEncodedStr)
	return cEncodedStr.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Util_EncodeId
func Java_io_v_syncbase_internal_Util_EncodeId(env *C.JNIEnv, cls C.jclass, obj C.jobject) C.jstring {
	cId := newVIdFromJava(env, obj)
	var cEncodedId C.v23_syncbase_String
	v23_syncbase_EncodeId(cId, &cEncodedId)
	return cEncodedId.extractToJava(env)
}

//export Java_io_v_syncbase_internal_Util_NamingJoin
func Java_io_v_syncbase_internal_Util_NamingJoin(env *C.JNIEnv, cls C.jclass, obj C.jobject) C.jstring {
	cElements := newVStringsFromJava(env, obj)
	var cStr C.v23_syncbase_String
	v23_syncbase_NamingJoin(cElements, &cStr)
	return cStr.extractToJava(env)
}

// The functions below are defined in this file and not in jni_types.go due to
// "inconsistent definitions" errors for various functions (C.NewObjectA and
// C.SetObjectField for example).

// All the extractToJava methods return Java types and deallocate all the
// pointers inside v23_syncbase_* variable.

func (x *C.v23_syncbase_AppPeer) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, neighborhoodPeerClass.class, neighborhoodPeerClass.init, nil)
	C.SetObjectField(env, obj, neighborhoodPeerClass.appName, x.appName.extractToJava(env))
	C.SetObjectField(env, obj, neighborhoodPeerClass.blessings, x.blessings.extractToJava(env))
	C.SetBooleanField(env, obj, neighborhoodPeerClass.isLost, x.isLost.extractToJava())
	return obj
}

func (x *C.v23_syncbase_ChangeType) extractToJava(env *C.JNIEnv) C.jobject {
	var obj C.jobject
	switch *x {
	case C.v23_syncbase_ChangeTypePut:
		obj = C.GetStaticObjectField(env, changeTypeClass.class, changeTypeClass.put)
	case C.v23_syncbase_ChangeTypeDelete:
		obj = C.GetStaticObjectField(env, changeTypeClass.class, changeTypeClass.delete)
	}
	return obj
}

func (x *C.v23_syncbase_EntityType) extractToJava(env *C.JNIEnv) C.jobject {
	var obj C.jobject
	switch *x {
	case C.v23_syncbase_EntityTypeRoot:
		obj = C.GetStaticObjectField(env, entityTypeClass.class, entityTypeClass.root)
	case C.v23_syncbase_EntityTypeCollection:
		obj = C.GetStaticObjectField(env, entityTypeClass.class, entityTypeClass.collection)
	case C.v23_syncbase_EntityTypeRow:
		obj = C.GetStaticObjectField(env, entityTypeClass.class, entityTypeClass.row)
	}
	return obj
}

func (x *C.v23_syncbase_Id) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, idClass.class, idClass.init, nil)
	C.SetObjectField(env, obj, idClass.blessing, x.blessing.extractToJava(env))
	C.SetObjectField(env, obj, idClass.name, x.name.extractToJava(env))
	x.free()
	return obj
}

// newVIds creates a v23_syncbase_Ids from a List<Id>.
func newVIdsFromJava(env *C.JNIEnv, obj C.jobject) C.v23_syncbase_Ids {
	if obj == nil {
		return C.v23_syncbase_Ids{}
	}
	listInterface := newJListInterface(env, obj)
	iterObj := C.CallObjectMethod(env, obj, listInterface.iterator)
	if C.ExceptionOccurred(env) != nil {
		panic("newVIds exception while trying to call List.iterator()")
	}

	iteratorInterface := newJIteratorInterface(env, iterObj)
	tmp := []C.v23_syncbase_Id{}
	for C.CallBooleanMethodA(env, iterObj, iteratorInterface.hasNext, nil) == C.JNI_TRUE {
		idObj := C.CallObjectMethod(env, iterObj, iteratorInterface.next)
		if C.ExceptionOccurred(env) != nil {
			panic("newVIds exception while trying to call Iterator.next()")
		}
		tmp = append(tmp, newVIdFromJava(env, idObj))
	}

	size := C.size_t(len(tmp)) * C.size_t(C.sizeof_v23_syncbase_Id)
	r := C.v23_syncbase_Ids{
		p: (*C.v23_syncbase_Id)(unsafe.Pointer(C.malloc(size))),
		n: C.int(len(tmp)),
	}
	for i := range tmp {
		*r.at(i) = tmp[i]
	}
	return r
}

func (x *C.v23_syncbase_Ids) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, arrayListClass.class, arrayListClass.init, nil)
	for i := 0; i < int(x.n); i++ {
		idObj := x.at(i).extractToJava(env)
		arg := *(*C.jvalue)(unsafe.Pointer(&idObj))
		C.CallBooleanMethodA(env, obj, arrayListClass.add, &arg)
	}
	x.free()
	return obj
}

func (x *C.v23_syncbase_Invite) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, syncgroupInviteClass.class, syncgroupInviteClass.init, nil)
	C.SetObjectField(env, obj, syncgroupInviteClass.syncgroup, x.syncgroup.extractToJava(env))
	C.SetObjectField(env, obj, syncgroupInviteClass.addresses, x.addresses.extractToJava(env))
	C.SetObjectField(env, obj, syncgroupInviteClass.blessingNames, x.blessingNames.extractToJava(env))
	return obj
}

func (x *C.v23_syncbase_KeyValue) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, keyValueClass.class, keyValueClass.init, nil)
	C.SetObjectField(env, obj, keyValueClass.key, x.key.extractToJava(env))
	C.SetObjectField(env, obj, keyValueClass.value, x.value.extractToJava(env))
	return obj
}

func (x *C.v23_syncbase_Permissions) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, permissionsClass.class, permissionsClass.init, nil)
	C.SetObjectField(env, obj, permissionsClass.json, x.json.extractToJava(env))
	return obj
}

// newVersionedPermissions extracts a VersionedPermissions object from a
// v23_syncbase_Permissions and a v23_syncbase_String version.
func newVersionedPermissions(env *C.JNIEnv, cPerms *C.v23_syncbase_Permissions, cVersion *C.v23_syncbase_String) C.jobject {
	obj := C.NewObjectA(env, versionedPermissionsClass.class, versionedPermissionsClass.init, nil)
	C.SetObjectField(env, obj, versionedPermissionsClass.version, cVersion.extractToJava(env))
	C.SetObjectField(env, obj, versionedPermissionsClass.permissions, cPerms.extractToJava(env))
	return obj
}

// newVStringsFromJava creates a v23_syncbase_Strings from a List<String>.
func newVStringsFromJava(env *C.JNIEnv, obj C.jobject) C.v23_syncbase_Strings {
	if obj == nil {
		return C.v23_syncbase_Strings{}
	}
	listInterface := newJListInterface(env, obj)
	iterObj := C.CallObjectMethod(env, obj, listInterface.iterator)
	if C.ExceptionOccurred(env) != nil {
		panic("newVStringsFromJava exception while trying to call List.iterator()")
	}

	iteratorInterface := newJIteratorInterface(env, iterObj)
	tmp := []C.v23_syncbase_String{}
	for C.CallBooleanMethodA(env, iterObj, iteratorInterface.hasNext, nil) == C.JNI_TRUE {
		stringObj := C.CallObjectMethod(env, iterObj, iteratorInterface.next)
		if C.ExceptionOccurred(env) != nil {
			panic("newVStringsFromJava exception while trying to call Iterator.next()")
		}
		tmp = append(tmp, newVStringFromJava(env, C.jstring(stringObj)))
	}

	size := C.size_t(len(tmp)) * C.size_t(C.sizeof_v23_syncbase_String)
	r := C.v23_syncbase_Strings{
		p: (*C.v23_syncbase_String)(unsafe.Pointer(C.malloc(size))),
		n: C.int(len(tmp)),
	}
	for i := range tmp {
		*r.at(i) = tmp[i]
	}
	return r
}

func (x *C.v23_syncbase_Strings) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, arrayListClass.class, arrayListClass.init, nil)
	for i := 0; i < int(x.n); i++ {
		s := x.at(i).extractToJava(env)
		arg := *(*C.jvalue)(unsafe.Pointer(&s))
		C.CallBooleanMethodA(env, obj, arrayListClass.add, &arg)
	}
	x.free()
	return obj
}

func (x *C.v23_syncbase_SyncgroupMemberInfo) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, syncgroupMemberInfoClass.class, syncgroupMemberInfoClass.init, nil)
	C.SetByteField(env, obj, syncgroupMemberInfoClass.syncPriority, C.jbyte(x.syncPriority))
	C.SetByteField(env, obj, syncgroupMemberInfoClass.blobDevType, C.jbyte(x.blobDevType))
	return obj
}

func (x *C.v23_syncbase_SyncgroupMemberInfoMap) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, hashMapClass.class, hashMapClass.init, nil)
	for i := 0; i < int(x.n); i++ {
		k, v := x.at(i)
		key := k.extractToJava(env)
		value := v.extractToJava(env)
		args := C.allocJValueArray(2)
		C.setJValueArrayElement(args, 0, *(*C.jvalue)(unsafe.Pointer(&key)))
		C.setJValueArrayElement(args, 1, *(*C.jvalue)(unsafe.Pointer(&value)))
		C.CallObjectMethodA(env, obj, hashMapClass.put, args)
		C.free(unsafe.Pointer(args))
	}
	x.free()
	return obj
}

func (x *C.v23_syncbase_SyncgroupSpec) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, syncgroupSpecClass.class, syncgroupSpecClass.init, nil)
	C.SetObjectField(env, obj, syncgroupSpecClass.description, x.description.extractToJava(env))
	C.SetObjectField(env, obj, syncgroupSpecClass.publishSyncbaseName, x.publishSyncbaseName.extractToJava(env))
	C.SetObjectField(env, obj, syncgroupSpecClass.permissions, x.perms.extractToJava(env))
	C.SetObjectField(env, obj, syncgroupSpecClass.collections, x.collections.extractToJava(env))
	C.SetObjectField(env, obj, syncgroupSpecClass.mountTables, x.mountTables.extractToJava(env))
	C.SetBooleanField(env, obj, syncgroupSpecClass.isPrivate, x.isPrivate.extractToJava())
	return obj
}

func (x *C.v23_syncbase_VError) extractToJava(env *C.JNIEnv) C.jobject {
	if x.id.p == nil {
		return nil
	}
	obj := C.NewObjectA(env, verrorClass.class, verrorClass.init, nil)
	C.SetObjectField(env, obj, verrorClass.id, x.id.extractToJava(env))
	C.SetLongField(env, obj, verrorClass.actionCode, C.jlong(x.actionCode))
	C.SetObjectField(env, obj, verrorClass.message, x.msg.extractToJava(env))
	C.SetObjectField(env, obj, verrorClass.stack, x.stack.extractToJava(env))
	x.free()
	return obj
}

func (x *C.v23_syncbase_WatchChange) extractToJava(env *C.JNIEnv) C.jobject {
	obj := C.NewObjectA(env, watchChangeClass.class, watchChangeClass.init, nil)
	C.SetObjectField(env, obj, watchChangeClass.entityType, x.entityType.extractToJava(env))
	C.SetObjectField(env, obj, watchChangeClass.collection, x.collection.extractToJava(env))
	C.SetObjectField(env, obj, watchChangeClass.row, x.row.extractToJava(env))
	C.SetObjectField(env, obj, watchChangeClass.changeType, x.changeType.extractToJava(env))
	C.SetObjectField(env, obj, watchChangeClass.value, x.value.extractToJava(env))
	C.SetObjectField(env, obj, watchChangeClass.resumeMarker, x.resumeMarker.extractToJava(env))
	C.SetBooleanField(env, obj, watchChangeClass.fromSync, x.fromSync.extractToJava())
	C.SetBooleanField(env, obj, watchChangeClass.continued, x.continued.extractToJava())
	return obj
}

// newVCollectionRowPatternsFromJava creates a
// v23_syncbase_CollectionRowPatterns from a List<CollectionRowPattern>.
func newVCollectionRowPatternsFromJava(env *C.JNIEnv, obj C.jobject) C.v23_syncbase_CollectionRowPatterns {
	if obj == nil {
		return C.v23_syncbase_CollectionRowPatterns{}
	}
	listInterface := newJListInterface(env, obj)
	iterObj := C.CallObjectMethod(env, obj, listInterface.iterator)
	if C.ExceptionOccurred(env) != nil {
		panic("newVCollectionRowPatternsFromJava exception while trying to call List.iterator()")
	}

	iteratorInterface := newJIteratorInterface(env, iterObj)
	tmp := []C.v23_syncbase_CollectionRowPattern{}
	for C.CallBooleanMethodA(env, iterObj, iteratorInterface.hasNext, nil) == C.JNI_TRUE {
		idObj := C.CallObjectMethod(env, iterObj, iteratorInterface.next)
		if C.ExceptionOccurred(env) != nil {
			panic("newVCollectionRowPatternsFromJava exception while trying to call Iterator.next()")
		}
		tmp = append(tmp, newVCollectionRowPatternFromJava(env, idObj))
	}

	size := C.size_t(len(tmp)) * C.size_t(C.sizeof_v23_syncbase_CollectionRowPattern)
	r := C.v23_syncbase_CollectionRowPatterns{
		p: (*C.v23_syncbase_CollectionRowPattern)(unsafe.Pointer(C.malloc(size))),
		n: C.int(len(tmp)),
	}
	for i := range tmp {
		*r.at(i) = tmp[i]
	}
	return r
}

func jFindClass(env *C.JNIEnv, name string) (C.jclass, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	class := C.FindClass(env, cName)
	if C.ExceptionOccurred(env) != nil {
		return nil, fmt.Errorf("couldn't find class %s", name)
	}

	globalRef := C.jclass(C.NewGlobalRef(env, class))
	if globalRef == nil {
		return nil, fmt.Errorf("couldn't allocate a global reference for class %s", name)
	}
	return globalRef, nil
}
