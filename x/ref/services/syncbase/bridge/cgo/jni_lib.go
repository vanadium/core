// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android
// +build cgo

package main

// #include <stdlib.h>
// #include "jni_wrapper.h"
// #include "lib.h"
import "C"

type jArrayListClass struct {
	class C.jclass
	init  C.jmethodID
	add   C.jmethodID
}

func newJArrayListClass(env *C.JNIEnv) jArrayListClass {
	cls, init := initClass(env, "java/util/ArrayList")
	return jArrayListClass{
		class: cls,
		init:  init,
		add:   jGetMethodID(env, cls, "add", "(Ljava/lang/Object;)Z"),
	}
}

type jCollectionRowPattern struct {
	class              C.jclass
	init               C.jmethodID
	collectionBlessing C.jfieldID
	collectionName     C.jfieldID
	rowKey             C.jfieldID
}

func newJCollectionRowPattern(env *C.JNIEnv) jCollectionRowPattern {
	cls, init := initClass(env, "io/v/syncbase/core/CollectionRowPattern")
	return jCollectionRowPattern{
		class:              cls,
		init:               init,
		collectionBlessing: jGetFieldID(env, cls, "collectionBlessing", "Ljava/lang/String;"),
		collectionName:     jGetFieldID(env, cls, "collectionName", "Ljava/lang/String;"),
		rowKey:             jGetFieldID(env, cls, "rowKey", "Ljava/lang/String;"),
	}
}

type jEntityType struct {
	class      C.jclass
	root       C.jfieldID
	collection C.jfieldID
	row        C.jfieldID
}

func newJEntityType(env *C.JNIEnv) jEntityType {
	cls := findClass(env, "io/v/syncbase/core/WatchChange$EntityType")
	return jEntityType{
		class:      cls,
		root:       jGetStaticFieldID(env, cls, "ROOT", "Lio/v/syncbase/core/WatchChange$EntityType;"),
		collection: jGetStaticFieldID(env, cls, "COLLECTION", "Lio/v/syncbase/core/WatchChange$EntityType;"),
		row:        jGetStaticFieldID(env, cls, "ROW", "Lio/v/syncbase/core/WatchChange$EntityType;"),
	}
}

type jHashMap struct {
	class C.jclass
	init  C.jmethodID
	put   C.jmethodID
}

func newJHashMap(env *C.JNIEnv) jHashMap {
	cls, init := initClass(env, "java/util/HashMap")
	return jHashMap{
		class: cls,
		init:  init,
		put:   jGetMethodID(env, cls, "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"),
	}
}

type jIdClass struct {
	class    C.jclass
	init     C.jmethodID
	blessing C.jfieldID
	name     C.jfieldID
}

func newJIdClass(env *C.JNIEnv) jIdClass {
	cls, init := initClass(env, "io/v/syncbase/core/Id")
	return jIdClass{
		class:    cls,
		init:     init,
		blessing: jGetFieldID(env, cls, "blessing", "Ljava/lang/String;"),
		name:     jGetFieldID(env, cls, "name", "Ljava/lang/String;"),
	}
}

type jIteratorInterface struct {
	hasNext C.jmethodID
	next    C.jmethodID
}

func newJIteratorInterface(env *C.JNIEnv, obj C.jobject) jIteratorInterface {
	cls := C.GetObjectClass(env, obj)
	return jIteratorInterface{
		hasNext: jGetMethodID(env, cls, "hasNext", "()Z"),
		next:    jGetMethodID(env, cls, "next", "()Ljava/lang/Object;"),
	}
}

type jKeyValue struct {
	class C.jclass
	init  C.jmethodID
	key   C.jfieldID
	value C.jfieldID
}

func newJKeyValue(env *C.JNIEnv) jKeyValue {
	cls, init := initClass(env, "io/v/syncbase/core/KeyValue")
	return jKeyValue{
		class: cls,
		init:  init,
		key:   jGetFieldID(env, cls, "key", "Ljava/lang/String;"),
		value: jGetFieldID(env, cls, "value", "[B"),
	}

}

type jListInterface struct {
	iterator C.jmethodID
	size     C.jmethodID
}

func newJListInterface(env *C.JNIEnv, obj C.jobject) jListInterface {
	cls := C.GetObjectClass(env, obj)
	return jListInterface{
		size:     jGetMethodID(env, cls, "size", "()I"),
		iterator: jGetMethodID(env, cls, "iterator", "()Ljava/util/Iterator;"),
	}
}

type jPermissions struct {
	class C.jclass
	init  C.jmethodID
	json  C.jfieldID
}

func newJPermissions(env *C.JNIEnv) jPermissions {
	cls, init := initClass(env, "io/v/syncbase/core/Permissions")
	return jPermissions{
		class: cls,
		init:  init,
		json:  jGetFieldID(env, cls, "json", "[B"),
	}
}

type jScanCallbacks struct {
	onKeyValue C.jmethodID
	onDone     C.jmethodID
}

func newJScanCallbacks(env *C.JNIEnv, obj C.jobject) jScanCallbacks {
	cls := C.GetObjectClass(env, obj)
	return jScanCallbacks{
		onKeyValue: jGetMethodID(env, cls, "onKeyValue", "(Lio/v/syncbase/core/KeyValue;)V"),
		onDone:     jGetMethodID(env, cls, "onDone", "(Lio/v/syncbase/core/VError;)V"),
	}
}

type jSyncgroupMemberInfo struct {
	class        C.jclass
	init         C.jmethodID
	syncPriority C.jfieldID
	blobDevType  C.jfieldID
}

func newJSyncgroupMemberInfo(env *C.JNIEnv) jSyncgroupMemberInfo {
	cls, init := initClass(env, "io/v/syncbase/core/SyncgroupMemberInfo")
	return jSyncgroupMemberInfo{
		class:        cls,
		init:         init,
		syncPriority: jGetFieldID(env, cls, "syncPriority", "I"),
		blobDevType:  jGetFieldID(env, cls, "blobDevType", "I"),
	}
}

type jSyncgroupSpec struct {
	class               C.jclass
	init                C.jmethodID
	description         C.jfieldID
	publishSyncbaseName C.jfieldID
	permissions         C.jfieldID
	collections         C.jfieldID
	mountTables         C.jfieldID
	isPrivate           C.jfieldID
}

func newJSyncgroupSpec(env *C.JNIEnv) jSyncgroupSpec {
	cls, init := initClass(env, "io/v/syncbase/core/SyncgroupSpec")
	return jSyncgroupSpec{
		class:               cls,
		init:                init,
		description:         jGetFieldID(env, cls, "description", "Ljava/lang/String;"),
		publishSyncbaseName: jGetFieldID(env, cls, "publishSyncbaseName", "Ljava/lang/String;"),
		permissions:         jGetFieldID(env, cls, "permissions", "Lio/v/syncbase/core/Permissions;"),
		collections:         jGetFieldID(env, cls, "collections", "Ljava/util/List;"),
		mountTables:         jGetFieldID(env, cls, "mountTables", "Ljava/util/List;"),
		isPrivate:           jGetFieldID(env, cls, "isPrivate", "Z"),
	}
}

type jVErrorClass struct {
	class      C.jclass
	init       C.jmethodID
	id         C.jfieldID
	actionCode C.jfieldID
	message    C.jfieldID
	stack      C.jfieldID
}

func newJVErrorClass(env *C.JNIEnv) jVErrorClass {
	cls, init := initClass(env, "io/v/syncbase/core/VError")
	return jVErrorClass{
		class:      cls,
		init:       init,
		id:         jGetFieldID(env, cls, "id", "Ljava/lang/String;"),
		actionCode: jGetFieldID(env, cls, "actionCode", "J"),
		message:    jGetFieldID(env, cls, "message", "Ljava/lang/String;"),
		stack:      jGetFieldID(env, cls, "stack", "Ljava/lang/String;"),
	}
}

type jVersionedPermissions struct {
	class       C.jclass
	init        C.jmethodID
	version     C.jfieldID
	permissions C.jfieldID
}

func newJVersionedPermissions(env *C.JNIEnv) jVersionedPermissions {
	cls, init := initClass(env, "io/v/syncbase/core/VersionedPermissions")
	return jVersionedPermissions{
		class:       cls,
		init:        init,
		version:     jGetFieldID(env, cls, "version", "Ljava/lang/String;"),
		permissions: jGetFieldID(env, cls, "permissions", "Lio/v/syncbase/core/Permissions;"),
	}
}

type jVersionedSyncgroupSpec struct {
	class         C.jclass
	init          C.jmethodID
	version       C.jfieldID
	syncgroupSpec C.jfieldID
}

func newJVersionedSyncgroupSpec(env *C.JNIEnv) jVersionedSyncgroupSpec {
	cls, init := initClass(env, "io/v/syncbase/core/VersionedSyncgroupSpec")
	return jVersionedSyncgroupSpec{
		class:         cls,
		init:          init,
		version:       jGetFieldID(env, cls, "version", "Ljava/lang/String;"),
		syncgroupSpec: jGetFieldID(env, cls, "syncgroupSpec", "Lio/v/syncbase/core/SyncgroupSpec;"),
	}
}

type jWatchChange struct {
	class        C.jclass
	init         C.jmethodID
	entityType   C.jfieldID
	collection   C.jfieldID
	row          C.jfieldID
	changeType   C.jfieldID
	value        C.jfieldID
	resumeMarker C.jfieldID
	fromSync     C.jfieldID
	continued    C.jfieldID
}

func newJWatchChange(env *C.JNIEnv) jWatchChange {
	cls, init := initClass(env, "io/v/syncbase/core/WatchChange")
	return jWatchChange{
		class:        cls,
		init:         init,
		entityType:   jGetFieldID(env, cls, "entityType", "Lio/v/syncbase/core/WatchChange$EntityType;"),
		collection:   jGetFieldID(env, cls, "collection", "Lio/v/syncbase/core/Id;"),
		row:          jGetFieldID(env, cls, "row", "Ljava/lang/String;"),
		changeType:   jGetFieldID(env, cls, "changeType", "Lio/v/syncbase/core/WatchChange$ChangeType;"),
		value:        jGetFieldID(env, cls, "value", "[B"),
		resumeMarker: jGetFieldID(env, cls, "resumeMarker", "[B"),
		fromSync:     jGetFieldID(env, cls, "fromSync", "Z"),
		continued:    jGetFieldID(env, cls, "continued", "Z"),
	}
}

type jWatchPatternsCallbacks struct {
	onChange C.jmethodID
	onError  C.jmethodID
}

func newJWatchPatternsCallbacks(env *C.JNIEnv, obj C.jobject) jWatchPatternsCallbacks {
	cls := C.GetObjectClass(env, obj)
	return jWatchPatternsCallbacks{
		onChange: jGetMethodID(env, cls, "onChange", "(Lio/v/syncbase/core/WatchChange;)V"),
		onError:  jGetMethodID(env, cls, "onError", "(Lio/v/syncbase/core/VError;)V"),
	}
}

type jChangeType struct {
	class  C.jclass
	put    C.jfieldID
	delete C.jfieldID
}

func newJChangeType(env *C.JNIEnv) jChangeType {
	cls := findClass(env, "io/v/syncbase/core/WatchChange$ChangeType")
	return jChangeType{
		class:  cls,
		put:    jGetStaticFieldID(env, cls, "PUT", "Lio/v/syncbase/core/WatchChange$ChangeType;"),
		delete: jGetStaticFieldID(env, cls, "DELETE", "Lio/v/syncbase/core/WatchChange$ChangeType;"),
	}
}

type jSyncgroupInvite struct {
	class         C.jclass
	init          C.jmethodID
	syncgroup     C.jfieldID
	addresses     C.jfieldID
	blessingNames C.jfieldID
}

func newJSyncgroupInvite(env *C.JNIEnv) jSyncgroupInvite {
	cls, init := initClass(env, "io/v/syncbase/core/SyncgroupInvite")
	return jSyncgroupInvite{
		class:         cls,
		init:          init,
		syncgroup:     jGetFieldID(env, cls, "syncgroup", "Lio/v/syncbase/core/Id;"),
		addresses:     jGetFieldID(env, cls, "addresses", "Ljava/util/List;"),
		blessingNames: jGetFieldID(env, cls, "blessingNames", "Ljava/util/List;"),
	}
}

type jSyncgroupInvitesCallbacks struct {
	onInvite C.jmethodID
}

func newJSyncgroupInvitesCallbacks(env *C.JNIEnv, obj C.jobject) jSyncgroupInvitesCallbacks {
	cls := C.GetObjectClass(env, obj)
	return jSyncgroupInvitesCallbacks{
		onInvite: jGetMethodID(env, cls, "onInvite", "(Lio/v/syncbase/core/SyncgroupInvite;)V"),
	}
}

type jNeighborhoodPeer struct {
	class     C.jclass
	init      C.jmethodID
	appName   C.jfieldID
	blessings C.jfieldID
	isLost    C.jfieldID
}

func newJNeighborhoodPeer(env *C.JNIEnv) jNeighborhoodPeer {
	cls, init := initClass(env, "io/v/syncbase/core/NeighborhoodPeer")
	return jNeighborhoodPeer{
		class:     cls,
		init:      init,
		appName:   jGetFieldID(env, cls, "appName", "Ljava/lang/String;"),
		blessings: jGetFieldID(env, cls, "blessings", "Ljava/lang/String;"),
		isLost:    jGetFieldID(env, cls, "isLost", "Z"),
	}
}

type jNeighborhoodScanCallbacks struct {
	onPeer C.jmethodID
}

func newJNeigbhorhoodScanCallbacks(env *C.JNIEnv, obj C.jobject) jNeighborhoodScanCallbacks {
	cls := C.GetObjectClass(env, obj)
	return jNeighborhoodScanCallbacks{
		onPeer: jGetMethodID(env, cls, "onPeer", "(Lio/v/syncbase/core/NeighborhoodPeer;)V"),
	}
}

// initClass returns the jclass and the jmethodID of the default constructor for
// a class.
func initClass(env *C.JNIEnv, name string) (C.jclass, C.jmethodID) {
	cls := findClass(env, name)
	return cls, jGetMethodID(env, cls, "<init>", "()V")
}

func findClass(env *C.JNIEnv, name string) C.jclass {
	cls, err := jFindClass(env, name)
	if err != nil {
		// The invariant is that we only deal with classes that must be
		// known to the JVM. A panic indicates a bug in our code.
		panic(err)
	}
	return cls
}
