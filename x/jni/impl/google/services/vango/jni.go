// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package vango

import (
	"fmt"
	"io"
	"runtime"
	"unsafe"

	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
)

// #include "jni.h"
import "C"

//export Java_io_v_android_util_Vango_nativeGoContextCall
func Java_io_v_android_util_Vango_nativeGoContextCall(jenv *C.JNIEnv, jVango C.jobject, jContext C.jobject, jKey C.jstring, jOutput C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	key := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jKey))))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	f, ok := vangoFuncs[key]
	if !ok {
		jutil.JThrowV(env, fmt.Errorf("vangoFunc key %q doesn't exist", key))
		return
	}
	if err := f(ctx, newJavaOutputWriter(env, jutil.Object(uintptr(unsafe.Pointer(jOutput))))); err != nil {
		jutil.JThrowV(env, err)
		return
	}

}

// javaOutputWriter translates an implementation of the Java Vango.OutputWriter interface to Go's io.Writer
func newJavaOutputWriter(env jutil.Env, o jutil.Object) io.Writer {
	ret := &javaOutputWriter{jutil.NewGlobalRef(env, o)}
	runtime.SetFinalizer(ret, func(w *javaOutputWriter) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jutil.DeleteGlobalRef(env, w.obj)
	})
	return ret
}

type javaOutputWriter struct {
	obj jutil.Object // implementation of the Java Vango.OutputWriter interface
}

func (w *javaOutputWriter) Write(b []byte) (int, error) {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	if err := jutil.CallVoidMethod(env, w.obj, "write", []jutil.Sign{jutil.StringSign}, string(b)); err != nil {
		return 0, err
	}
	return len(b), nil
}
