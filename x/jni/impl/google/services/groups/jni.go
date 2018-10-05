// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package groups

import (
	"os"
	"path/filepath"
	"unsafe"

	"v.io/v23"
	"v.io/x/ref/services/groups/lib"

	jrpc "v.io/x/jni/impl/google/rpc"
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
)

// #include "jni.h"
import "C"

var (
	contextSign       = jutil.ClassSign("io.v.v23.context.VContext")
	storageEngineSign = jutil.ClassSign("io.v.impl.google.services.groups.GroupServer$StorageEngine")
	serverSign        = jutil.ClassSign("io.v.v23.rpc.Server")

	jVRuntimeImplClass jutil.Class
)

// Init initializes the JNI code with the given Java environment.  This method
// must be invoked before any other method in this package and must be called
// from the main Java thread (e.g., On_Load()).
func Init(env jutil.Env) error {
	var err error
	jVRuntimeImplClass, err = jutil.JFindClass(env, "io/v/impl/google/rt/VRuntimeImpl")
	if err != nil {
		return err
	}
	return nil
}

//export Java_io_v_impl_google_services_groups_GroupServer_nativeWithNewServer
func Java_io_v_impl_google_services_groups_GroupServer_nativeWithNewServer(jenv *C.JNIEnv, jGroupServerClass C.jclass, jContext C.jobject, jGroupServerParams C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jCtx := jutil.Object(uintptr(unsafe.Pointer(jContext)))
	jParams := jutil.Object(uintptr(unsafe.Pointer(jGroupServerParams)))

	// Read and translate all of the server params.
	name, err := jutil.CallStringMethod(env, jParams, "getName", nil)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	rootDir, err := jutil.CallStringMethod(env, jParams, "getStorageRootDir", nil)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	if rootDir == "" {
		rootDir = filepath.Join(os.TempDir(), "groupserver")
		if err := os.Mkdir(rootDir, 0755); err != nil && !os.IsExist(err) {
			jutil.JThrowV(env, err)
			return 0
		}
	}
	jEngine, err := jutil.CallObjectMethod(env, jParams, "getStorageEngine", nil, storageEngineSign)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	engine, err := GoStorageEngine(env, jEngine)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}

	// Start the server.
	ctx, cancel, err := jcontext.GoContext(env, jCtx)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	dispatcher, err := lib.NewGroupsDispatcher(rootDir, engine)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	newCtx, s, err := v23.WithNewDispatchingServer(ctx, name, dispatcher)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jNewCtx, err := jcontext.JavaContext(env, newCtx, cancel)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jServer, err := jrpc.JavaServer(env, s)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	// Attach a server to the new context.
	jServerAttCtx, err := jutil.CallStaticObjectMethod(env, jVRuntimeImplClass, "withServer", []jutil.Sign{contextSign, serverSign}, contextSign, jNewCtx, jServer)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jServerAttCtx))
}
