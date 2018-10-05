// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package groups

import (
	"unsafe"

	"v.io/v23/security"
	"v.io/v23/services/groups"

	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
	jsecurity "v.io/x/jni/v23/security"
	jaccess "v.io/x/jni/v23/security/access"
)

// #include "jni.h"
import "C"

func Init(env jutil.Env) error {
	return nil
}

//export Java_io_v_v23_services_groups_PermissionsAuthorizer_nativeCreate
func Java_io_v_v23_services_groups_PermissionsAuthorizer_nativeCreate(jenv *C.JNIEnv, jPermissionsAuthorizerClass C.jclass, jPermissions C.jobject, jTagType C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	perms, err := jaccess.GoPermissions(env, jutil.Object(uintptr(unsafe.Pointer(jPermissions))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	tagType, err := jutil.GoVdlType(env, jutil.Object(uintptr(unsafe.Pointer(jTagType))))
	authorizer, err := groups.PermissionsAuthorizer(perms, tagType)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	ref := jutil.GoNewRef(&authorizer) // Un-refed when the Java PermissionsAuthorizer is finalized
	jAuthorizer, err := jutil.NewObject(env, jutil.Class(uintptr(unsafe.Pointer(jPermissionsAuthorizerClass))), []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jAuthorizer))
}

//export Java_io_v_v23_services_groups_PermissionsAuthorizer_nativeAuthorize
func Java_io_v_v23_services_groups_PermissionsAuthorizer_nativeAuthorize(jenv *C.JNIEnv, jPermissionsAuthorizer C.jobject, goRef C.jlong, jContext C.jobject, jCall C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	call, err := jsecurity.GoCall(env, jutil.Object(uintptr(unsafe.Pointer(jCall))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	if err := (*(*security.Authorizer)(jutil.GoRefValue(jutil.Ref(goRef)))).Authorize(ctx, call); err != nil {
		jutil.JThrowV(env, err)
		return
	}
}

//export Java_io_v_v23_services_groups_PermissionsAuthorizer_nativeFinalize
func Java_io_v_v23_services_groups_PermissionsAuthorizer_nativeFinalize(jenv *C.JNIEnv, jPermissionsAuthorizer C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}
