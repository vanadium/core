// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package context

import (
	"errors"
	"unsafe"

	"v.io/v23/context"
	jutil "v.io/x/jni/util"
)

// #include "jni.h"
// #include <stdlib.h>
import "C"

var (
	classSign      = jutil.ClassSign("java.lang.Class")
	doneReasonSign = jutil.ClassSign("io.v.v23.context.VContext$DoneReason")
	// Global reference for io.v.v23.context.VContext class.
	jVContextClass jutil.Class
	// Global reference for io.v.v23.context.VContext$DoneReason
	jDoneReasonClass jutil.Class
)

// Init initializes the JNI code with the given Java environment. This method
// must be called from the main Java thread.
func Init(env jutil.Env) error {
	// Cache global references to all Java classes used by the package.  This is
	// necessary because JNI gets access to the class loader only in the system
	// thread, so we aren't able to invoke FindClass in other threads.
	var err error
	jVContextClass, err = jutil.JFindClass(env, "io/v/v23/context/VContext")
	if err != nil {
		return err
	}
	jDoneReasonClass, err = jutil.JFindClass(env, "io/v/v23/context/VContext$DoneReason")
	if err != nil {
		return err
	}
	return nil
}

//export Java_io_v_v23_context_VContext_nativeCreate
func Java_io_v_v23_context_VContext_nativeCreate(jenv *C.JNIEnv, jVContext C.jclass) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _ := context.RootContext()
	jContext, err := JavaContext(env, ctx, nil)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jContext))
}

//export Java_io_v_v23_context_VContext_nativeCancel
func Java_io_v_v23_context_VContext_nativeCancel(jenv *C.JNIEnv, jVContext C.jobject, goCancelRef C.jlong) {
	(*(*context.CancelFunc)(jutil.GoRefValue(jutil.Ref(goCancelRef))))()
}

//export Java_io_v_v23_context_VContext_nativeIsCanceled
func Java_io_v_v23_context_VContext_nativeIsCanceled(jenv *C.JNIEnv, jVContext C.jobject, goRef C.jlong) C.jboolean {
	if (*(*context.T)(jutil.GoRefValue(jutil.Ref(goRef)))).Err() == nil {
		return C.JNI_FALSE
	}
	return C.JNI_TRUE
}

//export Java_io_v_v23_context_VContext_nativeDeadline
func Java_io_v_v23_context_VContext_nativeDeadline(jenv *C.JNIEnv, jVContext C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	d, ok := (*(*context.T)(jutil.GoRefValue(jutil.Ref(goRef)))).Deadline()
	if !ok {
		return 0
	}
	jDeadline, err := jutil.JTime(env, d)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jDeadline))
}

//export Java_io_v_v23_context_VContext_nativeOnDone
func Java_io_v_v23_context_VContext_nativeOnDone(jenv *C.JNIEnv, jVContext C.jobject, goRef C.jlong, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	ctx := (*(*context.T)(jutil.GoRefValue(jutil.Ref(goRef))))
	c := ctx.Done()
	if c == nil {
		jutil.CallbackOnFailure(env, jCallback, errors.New("Context isn't cancelable"))
		return
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		<-c
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jReason, err := JavaContextDoneReason(env, ctx.Err())
		if err != nil {
			return jutil.NullObject, err
		}
		// Must grab a global reference as we free up the env and all local references that come along
		// with it.
		return jutil.NewGlobalRef(env, jReason), nil // Un-refed in DoAsyncCall
	})
}

//export Java_io_v_v23_context_VContext_nativeValue
func Java_io_v_v23_context_VContext_nativeValue(jenv *C.JNIEnv, jVContext C.jobject, goRef C.jlong, jKeySign C.jstring) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	key := goContextKey(jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jKeySign)))))
	value := (*(*context.T)(jutil.GoRefValue(jutil.Ref(goRef)))).Value(key)
	jValue, err := JavaContextValue(env, value)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jValue))
}

//export Java_io_v_v23_context_VContext_nativeWithCancel
func Java_io_v_v23_context_VContext_nativeWithCancel(jenv *C.JNIEnv, jVContext C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, cancelFunc := context.WithCancel((*context.T)(jutil.GoRefValue(jutil.Ref(goRef))))
	jCtx, err := JavaContext(env, ctx, cancelFunc)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jCtx))
}

//export Java_io_v_v23_context_VContext_nativeWithDeadline
func Java_io_v_v23_context_VContext_nativeWithDeadline(jenv *C.JNIEnv, jVContext C.jobject, goRef C.jlong, jDeadline C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	deadline, err := jutil.GoTime(env, jutil.Object(uintptr(unsafe.Pointer(jDeadline))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	ctx, cancelFunc := context.WithDeadline((*context.T)(jutil.GoRefValue(jutil.Ref(goRef))), deadline)
	jCtx, err := JavaContext(env, ctx, cancelFunc)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jCtx))
}

//export Java_io_v_v23_context_VContext_nativeWithTimeout
func Java_io_v_v23_context_VContext_nativeWithTimeout(jenv *C.JNIEnv, jVContext C.jobject, goRef C.jlong, jTimeout C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	timeout, err := jutil.GoDuration(env, jutil.Object(uintptr(unsafe.Pointer(jTimeout))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	ctx, cancelFunc := context.WithTimeout((*context.T)(jutil.GoRefValue(jutil.Ref(goRef))), timeout)
	jCtx, err := JavaContext(env, ctx, cancelFunc)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jCtx))
}

//export Java_io_v_v23_context_VContext_nativeWithValue
func Java_io_v_v23_context_VContext_nativeWithValue(jenv *C.JNIEnv, jVContext C.jobject, goRef C.jlong, goCancelRef C.jlong, jKeySign C.jstring, jValue C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	key := goContextKey(jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jKeySign)))))
	value, err := GoContextValue(env, jutil.Object(uintptr(unsafe.Pointer(jValue))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	ctx := context.WithValue((*context.T)(jutil.GoRefValue(jutil.Ref(goRef))), key, value)
	var cancel context.CancelFunc
	if goCancelRef != 0 {
		cancel = (*(*context.CancelFunc)(jutil.GoRefValue(jutil.Ref(goCancelRef))))
	}
	jCtx, err := JavaContext(env, ctx, cancel)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jCtx))
}

//export Java_io_v_v23_context_VContext_nativeFinalize
func Java_io_v_v23_context_VContext_nativeFinalize(jenv *C.JNIEnv, jVContext C.jobject, goRef C.jlong, goCancelRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
	if jutil.Ref(goCancelRef) != jutil.NullRef {
		jutil.GoDecRef(jutil.Ref(goCancelRef))
	}

}
