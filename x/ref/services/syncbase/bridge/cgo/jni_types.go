// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains JNI conversions to/from Java types for the types declared
// in lib.h. The other file that contains functions related to the types in
// lib.h is types.go. We need to place the JNI code in a separate file in order
// to use build tags to restrict the compilation to Java and Android.
//
// All "x.extractToJava" methods leave "x" in the same state as "x.free".

// +build java android
// +build cgo

package main

import (
	"fmt"
	"unsafe"
)

// #include <stdlib.h>
// #include "jni_wrapper.h"
// #include "lib.h"
import "C"

// All the extractToJava methods return Java types and deallocate all the
// pointers inside v23_syncbase_* variable.

// extractToJava constructs a jboolean from a bool.
func (x *C.bool) extractToJava() C.jboolean {
	if *x == false {
		return 0
	}
	return 1
}

func newVBytesFromJava(env *C.JNIEnv, array C.jbyteArray) C.v23_syncbase_Bytes {
	r := C.v23_syncbase_Bytes{}
	if array == nil {
		return r
	}
	n := C.GetArrayLength(env, array)
	r.n = C.int(n)
	r.p = (*C.uint8_t)(C.malloc(C.size_t(r.n)))
	C.GetByteArrayRegion(env, array, 0, n, (*C.jbyte)(unsafe.Pointer(r.p)))
	// We don't have to check for exceptions because GetByteArrayRegion can
	// only throw ArrayIndexOutOfBoundsException and we know the requested
	// amount of elements is valid.
	return r
}

func (x *C.v23_syncbase_Bytes) extractToJava(env *C.JNIEnv) C.jbyteArray {
	if x.n < 0 {
		panic(fmt.Sprintf("bad size in C.v23_syncbase_Bytes: %d", x.n))
	}
	if x.n == 0 {
		return nil
	}
	obj := C.NewByteArray(env, C.jsize(x.n))
	if C.ExceptionOccurred(env) != nil {
		panic("NewByteArray OutOfMemoryError exception")
	}
	C.SetByteArrayRegion(env, obj, 0, C.jsize(x.n), (*C.jbyte)(unsafe.Pointer(x.p)))
	// We don't have to check for exceptions because SetByteArrayRegion can
	// only throw ArrayIndexOutOfBoundsException and we know the requested
	// amount of elements is valid.
	x.free()
	return obj
}

// newVIdFromJava creates a v23_syncbase_Id from an Id object.
func newVIdFromJava(env *C.JNIEnv, obj C.jobject) C.v23_syncbase_Id {
	blessing := C.jstring(C.GetObjectField(env, obj, idClass.blessing))
	if C.ExceptionOccurred(env) != nil {
		panic("newVIdFromJava exception while retrieving Id.blessing")
	}

	name := C.jstring(C.GetObjectField(env, obj, idClass.name))
	if C.ExceptionOccurred(env) != nil {
		panic("newVIdFromJava exception while retrieving Id.name")
	}

	return C.v23_syncbase_Id{
		blessing: newVStringFromJava(env, blessing),
		name:     newVStringFromJava(env, name),
	}
}

// extractToJava constructs a jstring from a v23_syncbase_String. The pointer
// inside v23_syncbase_String will be freed. The code is somewhat complicated
// and inefficient because the NewStringUTF from JNI only works with modified
// UTF-8 strings (inner nulls are encoded as 0xC0, 0x80 and the string is
// terminated with a null).
func (x *C.v23_syncbase_String) extractToJava(env *C.JNIEnv) C.jstring {
	if x.p == nil {
		return nil
	}
	n := int(x.n)
	srcPtr := uintptr(unsafe.Pointer(x.p))
	numNulls := 0
	for i := 0; i < n; i++ {
		if *(*byte)(unsafe.Pointer(srcPtr + uintptr(i))) == 0 {
			numNulls++
		}
	}
	tmp := C.malloc(C.size_t(n + numNulls + 1))
	defer C.free(tmp)
	tmpPtr := uintptr(tmp)
	j := 0
	for i := 0; i < n; i, j = i+1, j+1 {
		if *(*byte)(unsafe.Pointer(srcPtr + uintptr(i))) != 0 {
			*(*byte)(unsafe.Pointer(tmpPtr + uintptr(j))) = *(*byte)(unsafe.Pointer(srcPtr + uintptr(i)))
			continue
		}
		*(*byte)(unsafe.Pointer(tmpPtr + uintptr(j))) = 0xC0
		j++
		*(*byte)(unsafe.Pointer(tmpPtr + uintptr(j))) = 0x80
	}
	*(*byte)(unsafe.Pointer(tmpPtr + uintptr(j))) = 0
	r := C.NewStringUTF(env, (*C.char)(tmp))
	if C.ExceptionOccurred(env) != nil {
		panic("NewStringUTF OutOfMemoryError exception")
	}
	x.free()
	return r
}

func newVStringFromJava(env *C.JNIEnv, s C.jstring) C.v23_syncbase_String {
	r := C.v23_syncbase_String{}
	if s == nil {
		return r
	}
	// Note that GetStringUTFLength does not include a trailing zero.
	n := int(C.GetStringUTFLength(env, s))
	r.n = C.int(n)
	// TODO(razvanm): The JNI documentation doesn't clearly specify whether
	// the string returned by GetStringUTFRegion is null-terminated. What
	// I empirically found was that the heap gets corrupted if we allocate
	// the exact amount of bytes and the string size is of certain sizes (to
	// be more specific, size of the form 24 + 16 * x). Adding a single
	// extra byte seems to avoid the issue. I only checked sizes up to 32K.
	//
	// Some interesting perspective on the JNI's brokenness:
	//   http://www.club.cc.cmu.edu/~cmccabe/blog_jni_flaws.html
	r.p = (*C.char)(C.malloc(C.size_t(r.n + 1)))
	p := uintptr(unsafe.Pointer(r.p))
	// Note that we need to use GetStringLength and not GetStringUTFLength
	// because we need to indicate how many Unicode characters we want to be
	// copied.
	C.GetStringUTFRegion(env, s, 0, C.GetStringLength(env, s), r.p)
	// We don't have to check for exceptions because GetStringUTFRegion can
	// only throw StringIndexOutOfBoundsException and we know the requested
	// amount of characters is valid.
	j := 0
	for i := 0; i < n; i, j = i+1, j+1 {
		if i+1 < n && *(*byte)(unsafe.Pointer(p + uintptr(i))) == 0xC0 && *(*byte)(unsafe.Pointer(p + uintptr(i+1))) == 0x80 {
			*(*byte)(unsafe.Pointer(p + uintptr(j))) = 0
			i++
			continue
		}

		if j == i {
			continue
		}

		*(*byte)(unsafe.Pointer(p + uintptr(j))) = *(*byte)(unsafe.Pointer(p + uintptr(i)))
	}
	r.p = (*C.char)(C.realloc(unsafe.Pointer(r.p), (C.size_t)(j)))
	return r
}

// newVSyncgroupMemberInfoFromJava creates a v23_syncbase_SyncgroupMemberInfo
// from a SyncgroupMemberInfo object.
func newVSyncgroupMemberInfoFromJava(env *C.JNIEnv, obj C.jobject) C.v23_syncbase_SyncgroupMemberInfo {
	syncPriority := C.GetIntField(env, obj, syncgroupMemberInfoClass.syncPriority)
	if C.ExceptionOccurred(env) != nil {
		panic("newVSyncgroupMemberInfoFromJava exception while retrieving SyncgroupMemberInfo.syncPriority")
	}
	blobDevType := C.GetIntField(env, obj, syncgroupMemberInfoClass.blobDevType)
	if C.ExceptionOccurred(env) != nil {
		panic("newVSyncgroupMemberInfoFromJava exception while retrieving SyncgroupMemberInfo.blobDevType")
	}
	return C.v23_syncbase_SyncgroupMemberInfo{
		syncPriority: C.uint8_t(syncPriority),
		blobDevType:  C.uint8_t(blobDevType),
	}
}

// newVSyngroupSpecFromJava creates a v23_syncbase_SyncgroupSpec from a
// SyncgroupSpec object.
func newVSyngroupSpecFromJava(env *C.JNIEnv, obj C.jobject) C.v23_syncbase_SyncgroupSpec {
	description := C.jstring(C.GetObjectField(env, obj, syncgroupSpecClass.description))
	if C.ExceptionOccurred(env) != nil {
		panic("newVSyngroupSpecFromJava exception while retrieving SyncgroupSpec.description")
	}

	publishSyncbaseName := C.jstring(C.GetObjectField(env, obj, syncgroupSpecClass.publishSyncbaseName))
	if C.ExceptionOccurred(env) != nil {
		panic("newVSyngroupSpecFromJava exception while retrieving SyncgroupSpec.publishSyncbaseName")
	}

	permissions := C.GetObjectField(env, obj, syncgroupSpecClass.permissions)
	if C.ExceptionOccurred(env) != nil {
		panic("newVSyngroupSpecFromJava exception while retrieving SyncgroupSpec.permissions")
	}

	collections := C.GetObjectField(env, obj, syncgroupSpecClass.collections)
	if C.ExceptionOccurred(env) != nil {
		panic("newVSyngroupSpecFromJava exception while retrieving SyncgroupSpec.collections")
	}

	mountTables := C.GetObjectField(env, obj, syncgroupSpecClass.mountTables)
	if C.ExceptionOccurred(env) != nil {
		panic("newVSyngroupSpecFromJava exception while retrieving SyncgroupSpec.mountTables")
	}

	return C.v23_syncbase_SyncgroupSpec{
		description:         newVStringFromJava(env, description),
		publishSyncbaseName: newVStringFromJava(env, publishSyncbaseName),
		perms:               newVPermissionsFromJava(env, permissions),
		collections:         newVIdsFromJava(env, collections),
		mountTables:         newVStringsFromJava(env, mountTables),
	}
}

// newVSyngroupSpecAndVersionFromJava creates a v23_syncbase_SyncgroupSpec and
// v23_syncbase_String version string from a VersionedSyncgroupSpec object.
func newVSyngroupSpecAndVersionFromJava(env *C.JNIEnv, obj C.jobject) (C.v23_syncbase_SyncgroupSpec, C.v23_syncbase_String) {
	version := C.jstring(C.GetObjectField(env, obj, versionedSyncgroupSpecClass.version))
	if C.ExceptionOccurred(env) != nil {
		panic("newVSyngroupSpecAndVersionFromJava exception while retrieving VersionedSyncgroupSpec.version")
	}
	cSpec := newVSyngroupSpecFromJava(env, C.GetObjectField(env, obj, versionedSyncgroupSpecClass.syncgroupSpec))
	if C.ExceptionOccurred(env) != nil {
		panic("newVSyngroupSpecAndVersionFromJava exception while retrieving VersionedSyncgroupSpec.syncgroupSpec")
	}
	return cSpec, newVStringFromJava(env, version)
}

// newVPermissionsFromJava creates a v23_syncbase_Permissions from a Permissions
// object.
func newVPermissionsFromJava(env *C.JNIEnv, obj C.jobject) C.v23_syncbase_Permissions {
	if obj == nil {
		return C.v23_syncbase_Permissions{}
	}
	return C.v23_syncbase_Permissions{
		json: newVBytesFromJava(env, C.jbyteArray(C.GetObjectField(env, obj, permissionsClass.json))),
	}
}

// newVPermissionsAndVersionFromJava creates a v23_syncbase_Permissions and
// v23_syncbase_String version string from a VersionedPermissions object.
func newVPermissionsAndVersionFromJava(env *C.JNIEnv, obj C.jobject) (C.v23_syncbase_Permissions, C.v23_syncbase_String) {
	version := C.jstring(C.GetObjectField(env, obj, versionedPermissionsClass.version))
	if C.ExceptionOccurred(env) != nil {
		panic("newVPermissionsAndVersionFromJava exception while retrieving VersionedPermissions.version")
	}
	cPerms := newVPermissionsFromJava(env, C.GetObjectField(env, obj, versionedPermissionsClass.permissions))
	if C.ExceptionOccurred(env) != nil {
		panic("newVPermissionsAndVersionFromJava exception while retrieving VersionedPermissions.permissions")
	}
	return cPerms, newVStringFromJava(env, version)
}

// newVCollectionRowPatternFromJava creates a v23_syncbase_CollectionRowPattern
// from a CollectionRowPattern object.
func newVCollectionRowPatternFromJava(env *C.JNIEnv, obj C.jobject) C.v23_syncbase_CollectionRowPattern {
	collectionBlessing := C.jstring(C.GetObjectField(env, obj, collectionRowPatternClass.collectionBlessing))
	if C.ExceptionOccurred(env) != nil {
		panic("newVCollectionRowPatternFromJava exception while retrieving CollectionRowPattern.collectionBlessing")
	}

	collectionName := C.jstring(C.GetObjectField(env, obj, collectionRowPatternClass.collectionName))
	if C.ExceptionOccurred(env) != nil {
		panic("newVCollectionRowPatternFromJava exception while retrieving CollectionRowPattern.collectionName")
	}

	rowKey := C.jstring(C.GetObjectField(env, obj, collectionRowPatternClass.rowKey))
	if C.ExceptionOccurred(env) != nil {
		panic("newVCollectionRowPatternFromJava exception while retrieving CollectionRowPattern.rowKey")
	}

	return C.v23_syncbase_CollectionRowPattern{
		collectionBlessing: newVStringFromJava(env, collectionBlessing),
		collectionName:     newVStringFromJava(env, collectionName),
		rowKey:             newVStringFromJava(env, rowKey),
	}
}
