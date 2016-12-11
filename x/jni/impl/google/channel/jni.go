// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package channel

import (
	"unsafe"

	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

var (
	contextSign = jutil.ClassSign("io.v.v23.context.VContext")
	// Global reference for io.v.impl.google.channel.InputChannelImpl class.
	jInputChannelImplClass jutil.Class
	// Global reference for io.v.impl.google.channel.OutputChannelImpl class.
	jOutputChannelImplClass jutil.Class
)

// Init initializes the JNI code with the given Java environment.  This method
// must be invoked before any other method in this package and must be called
// from the main Java thread (e.g., On_Load()).
func Init(env jutil.Env) error {
	var err error
	jInputChannelImplClass, err = jutil.JFindClass(env, "io/v/impl/google/channel/InputChannelImpl")
	if err != nil {
		return err
	}
	jOutputChannelImplClass, err = jutil.JFindClass(env, "io/v/impl/google/channel/OutputChannelImpl")
	if err != nil {
		return err
	}
	return nil
}

//export Java_io_v_impl_google_channel_InputChannelImpl_nativeRecv
func Java_io_v_impl_google_channel_InputChannelImpl_nativeRecv(jenv *C.JNIEnv, jInputChannelImpl C.jobject, goRecvRef C.jlong, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	recv := *(*func() (jutil.Object, error))(jutil.GoRefValue(jutil.Ref(goRecvRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	jutil.DoAsyncCall(env, jCallback, recv)
}

//export Java_io_v_impl_google_channel_InputChannelImpl_nativeFinalize
func Java_io_v_impl_google_channel_InputChannelImpl_nativeFinalize(jenv *C.JNIEnv, jInputChannelImpl C.jobject, goRecvRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRecvRef))
}

//export Java_io_v_impl_google_channel_OutputChannelImpl_nativeSend
func Java_io_v_impl_google_channel_OutputChannelImpl_nativeSend(jenv *C.JNIEnv, jOutputChannelClass C.jclass, goConvertRef C.jlong, goSendRef C.jlong, jItemObj C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	convert := *(*func(jutil.Object) (interface{}, error))(jutil.GoRefValue(jutil.Ref(goConvertRef)))
	send := *(*func(interface{}) error)(jutil.GoRefValue(jutil.Ref(goSendRef)))
	jItem := jutil.Object(uintptr(unsafe.Pointer(jItemObj)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	// NOTE(spetrovic): Conversion must be done outside of DoAsyncCall as it references a Java
	// object.
	item, err := convert(jItem)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return jutil.NullObject, send(item)
	})
}

//export Java_io_v_impl_google_channel_OutputChannelImpl_nativeClose
func Java_io_v_impl_google_channel_OutputChannelImpl_nativeClose(jenv *C.JNIEnv, jOutputChannelClass C.jclass, goCloseRef C.jlong, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	close := *(*func() error)(jutil.GoRefValue(jutil.Ref(goCloseRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return jutil.NullObject, close()
	})
}

//export Java_io_v_impl_google_channel_OutputChannelImpl_nativeFinalize
func Java_io_v_impl_google_channel_OutputChannelImpl_nativeFinalize(jenv *C.JNIEnv, jOutputChannelClass C.jclass, goConvertRef C.jlong, goSendRef C.jlong, goCloseRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goConvertRef))
	jutil.GoDecRef(jutil.Ref(goSendRef))
	jutil.GoDecRef(jutil.Ref(goCloseRef))
}
