// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package mounttable

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"unsafe"

	"v.io/v23"
	"v.io/v23/security/access"
	"v.io/x/ref/services/mounttable/mounttablelib"

	"v.io/v23/options"
	jrpc "v.io/x/jni/impl/google/rpc"
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
	jaccess "v.io/x/jni/v23/security/access"
)

// #include "jni.h"
import "C"

var (
	permissionsSign = jutil.ClassSign("io.v.v23.security.access.Permissions")
	contextSign     = jutil.ClassSign("io.v.v23.context.VContext")
	serverSign      = jutil.ClassSign("io.v.v23.rpc.Server")

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

//export Java_io_v_impl_google_services_mounttable_MountTableServer_nativeWithNewServer
func Java_io_v_impl_google_services_mounttable_MountTableServer_nativeWithNewServer(jenv *C.JNIEnv, jMountTableServerClass C.jclass, jContext C.jobject, jMountTableServerParams C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jCtx := jutil.Object(uintptr(unsafe.Pointer(jContext)))
	jParams := jutil.Object(uintptr(unsafe.Pointer(jMountTableServerParams)))

	// Read and translate all of the server params.
	mountName, err := jutil.CallStringMethod(env, jParams, "getName", nil)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	rootDir, err := jutil.CallStringMethod(env, jParams, "getStorageRootDir", nil)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	permsJMap, err := jutil.CallMapMethod(env, jParams, "getPermissions", nil)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	permsMap := make(map[string]access.Permissions)
	for jPath, jPerms := range permsJMap {
		path := jutil.GoString(env, jPath)
		perms, err := jaccess.GoPermissions(env, jPerms)
		if err != nil {
			jutil.JThrowV(env, err)
			return 0
		}
		permsMap[path] = perms
	}
	// Write JSON-encoded permissions to a file.
	jsonPerms, err := json.Marshal(permsMap)
	if err != nil {
		jutil.JThrowV(env, fmt.Errorf("Couldn't JSON-encode path-permissions: %v", err))
		return 0

	}
	permsFile, err := ioutil.TempFile(rootDir, "jni_permissions")
	if err != nil {
		jutil.JThrowV(env, fmt.Errorf("Couldn't create permissions file: %v", err))
		return 0
	}
	w := bufio.NewWriter(permsFile)
	if _, err := w.Write(jsonPerms); err != nil {
		jutil.JThrowV(env, fmt.Errorf("Couldn't write to permissions file: %v", err))
		return 0
	}
	if err := w.Flush(); err != nil {
		jutil.JThrowV(env, fmt.Errorf("Couldn't flush to permissions file: %v", err))
	}
	statsPrefix, err := jutil.CallStringMethod(env, jParams, "getStatsPrefix", nil)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}

	// Start the mounttable server.
	ctx, cancel, err := jcontext.GoContext(env, jCtx)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	d, err := mounttablelib.NewMountTableDispatcher(ctx, permsFile.Name(), rootDir, statsPrefix)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	newCtx, s, err := v23.WithNewDispatchingServer(ctx, mountName, d, options.ServesMountTable(true))
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
