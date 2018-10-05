// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package access

import (
	"unsafe"

	"v.io/v23/security"
	"v.io/v23/security/access"

	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
	jsecurity "v.io/x/jni/v23/security"
)

// #include "jni.h"
import "C"

var (
	// Global reference for io.v.v23.security.access.AccessList class.
	jAccessListClass jutil.Class
	// Global reference for io.v.v23.security.access.Permissions class.
	jPermissionsClass jutil.Class
)

func Init(env jutil.Env) error {
	var err error
	jAccessListClass, err = jutil.JFindClass(env, "io/v/v23/security/access/AccessList")
	if err != nil {
		return err
	}
	jPermissionsClass, err = jutil.JFindClass(env, "io/v/v23/security/access/Permissions")
	if err != nil {
		return err
	}
	return nil
}

//export Java_io_v_v23_security_access_AccessList_nativeCreate
func Java_io_v_v23_security_access_AccessList_nativeCreate(jenv *C.JNIEnv, jAccessList C.jobject) C.jlong {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	acl, err := GoAccessList(env, jutil.Object(uintptr(unsafe.Pointer(jAccessList))))
	if err != nil {
		jutil.JThrowV(env, err)
		return C.jlong(0)
	}
	ref := jutil.GoNewRef(&acl) // Un-refed when the AccessList object is finalized
	return C.jlong(ref)
}

//export Java_io_v_v23_security_access_AccessList_nativeIncludes
func Java_io_v_v23_security_access_AccessList_nativeIncludes(jenv *C.JNIEnv, jAccessList C.jobject, goRef C.jlong, jBlessings C.jobjectArray) C.jboolean {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessings, err := jutil.GoStringArray(env, jutil.Object(uintptr(unsafe.Pointer(jBlessings))))
	if err != nil {
		jutil.JThrowV(env, err)
		return C.JNI_FALSE
	}
	ok := (*(*access.AccessList)(jutil.GoRefValue(jutil.Ref(goRef)))).Includes(blessings...)
	if ok {
		return C.JNI_TRUE
	}
	return C.JNI_FALSE
}

//export Java_io_v_v23_security_access_AccessList_nativeAuthorize
func Java_io_v_v23_security_access_AccessList_nativeAuthorize(jenv *C.JNIEnv, jAccessList C.jobject, goRef C.jlong, jCtx C.jobject, jCall C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jCtx))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	call, err := jsecurity.GoCall(env, jutil.Object(uintptr(unsafe.Pointer(jCall))))
	if err != nil {
		jutil.JThrowV(env, err)
	}
	if err := (*(*access.AccessList)(jutil.GoRefValue(jutil.Ref(goRef)))).Authorize(ctx, call); err != nil {
		jutil.JThrowV(env, err)
		return
	}
}

//export Java_io_v_v23_security_access_AccessList_nativeFinalize
func Java_io_v_v23_security_access_AccessList_nativeFinalize(jenv *C.JNIEnv, jAccessList C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_v23_security_access_PermissionsAuthorizer_nativeCreate
func Java_io_v_v23_security_access_PermissionsAuthorizer_nativeCreate(jenv *C.JNIEnv, jPermissionsAuthorizerClass C.jclass, jPermissions C.jobject, jTagType C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	perms, err := GoPermissions(env, jutil.Object(uintptr(unsafe.Pointer(jPermissions))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	tagType, err := jutil.GoVdlType(env, jutil.Object(uintptr(unsafe.Pointer(jTagType))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	authorizer, err := access.PermissionsAuthorizer(perms, tagType)
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

//export Java_io_v_v23_security_access_PermissionsAuthorizer_nativeAuthorize
func Java_io_v_v23_security_access_PermissionsAuthorizer_nativeAuthorize(jenv *C.JNIEnv, jPermissionsAuthorizer C.jobject, goRef C.jlong, jContext C.jobject, jCall C.jobject) {
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

//export Java_io_v_v23_security_access_PermissionsAuthorizer_nativeFinalize
func Java_io_v_v23_security_access_PermissionsAuthorizer_nativeFinalize(jenv *C.JNIEnv, jPermissionsAuthorizer C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}
