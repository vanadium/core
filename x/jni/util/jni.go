// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package util

import (
	"fmt"
	"unsafe"
)

// #include "jni_wrapper.h"
import "C"

var (
	callbackSign = ClassSign("io.v.v23.rpc.Callback")
	futureSign   = ClassSign("com.google.common.util.concurrent.ListenableFuture")
	// Global reference for io.v.util.Util class.
	jUtilClass Class
	// Global reference for io.v.v23.Options class.
	jOptionsClass Class
	// Global reference for io.v.v23.vom.VomUtil class.
	jVomUtilClass Class
	// Global reference for io.v.v23.verror.VException class.
	jVExceptionClass Class
	// Global reference for io.v.v23.verror.VException$ActionCode class.
	jActionCodeClass Class
	// Global reference for io.v.v23.verror.VException$IDAction class.
	jIDActionClass Class
	// Global reference for io.v.v23.vdl.VdlValue class.
	jVdlValueClass Class
	// Global reference for io.v.v23.vdl.VdlTypeObject class.
	jVdlTypeObjectClass Class
	// Global reference for org.joda.time.DateTime class.
	jDateTimeClass Class
	// Global reference for org.joda.time.Duration class.
	jDurationClass Class
	// Global reference for java.util.Arrays class
	jArraysClass Class
	// Global reference for java.util.ArrayList class.
	jArrayListClass Class
	// Global reference for java.lang.Throwable class.
	jThrowableClass Class
	// Global reference for java.lang.System class.
	jSystemClass Class
	// Global reference for java.lang.Object class.
	jObjectClass Class
	// Global reference for java.lang.Boolean class.
	jBooleanClass Class
	// Global reference for java.lang.Integer class.
	jIntegerClass Class
	// Global reference for java.lang.String class.
	jStringClass Class
	// Global reference for java.util.HashMap class.
	jHashMapClass Class
	// Global reference for com.google.common.collect.HashMultimap class.
	jHashMultimapClass Class
	// Global reference for []byte class.
	jByteArrayClass Class
	// Global reference for io.v.util.NativeCallback class.
	jNativeCallbackClass Class
	// Cached Java VM.
	jVM *C.JavaVM
)

// Init initializes the JNI code with the given Java environment.  This method
// must be invoked before any other method in this package and must be called
// from the main Java thread (e.g., On_Load()).
func Init(env Env) error {
	var err error
	jUtilClass, err = JFindClass(env, "io/v/util/Util")
	if err != nil {
		return err
	}
	jOptionsClass, err = JFindClass(env, "io/v/v23/Options")
	if err != nil {
		return err
	}
	jVomUtilClass, err = JFindClass(env, "io/v/v23/vom/VomUtil")
	if err != nil {
		return err
	}
	jVExceptionClass, err = JFindClass(env, "io/v/v23/verror/VException")
	if err != nil {
		return err
	}
	jActionCodeClass, err = JFindClass(env, "io/v/v23/verror/VException$ActionCode")
	if err != nil {
		return err
	}
	jIDActionClass, err = JFindClass(env, "io/v/v23/verror/VException$IDAction")
	if err != nil {
		return err
	}
	jVdlValueClass, err = JFindClass(env, "io/v/v23/vdl/VdlValue")
	if err != nil {
		return err
	}
	jVdlTypeObjectClass, err = JFindClass(env, "io/v/v23/vdl/VdlTypeObject")
	if err != nil {
		return err
	}
	jDateTimeClass, err = JFindClass(env, "org/joda/time/DateTime")
	if err != nil {
		return err
	}
	jDurationClass, err = JFindClass(env, "org/joda/time/Duration")
	if err != nil {
		return err
	}
	jArraysClass, err = JFindClass(env, "java/util/Arrays")
	if err != nil {
		return err
	}
	jArrayListClass, err = JFindClass(env, "java/util/ArrayList")
	if err != nil {
		return err
	}
	jThrowableClass, err = JFindClass(env, "java/lang/Throwable")
	if err != nil {
		return err
	}
	jSystemClass, err = JFindClass(env, "java/lang/System")
	if err != nil {
		return err
	}
	jObjectClass, err = JFindClass(env, "java/lang/Object")
	if err != nil {
		return err
	}
	jBooleanClass, err = JFindClass(env, "java/lang/Boolean")
	if err != nil {
		return err
	}
	jIntegerClass, err = JFindClass(env, "java/lang/Integer")
	if err != nil {
		return err
	}
	jStringClass, err = JFindClass(env, "java/lang/String")
	if err != nil {
		return err
	}
	jHashMapClass, err = JFindClass(env, "java/util/HashMap")
	if err != nil {
		return err
	}
	jHashMultimapClass, err = JFindClass(env, "com/google/common/collect/HashMultimap")
	if err != nil {
		return err
	}
	jByteArrayClass, err = JFindClass(env, "[B")
	if err != nil {
		return err
	}
	jNativeCallbackClass, err = JFindClass(env, "io/v/util/NativeCallback")
	if err != nil {
		return err
	}
	if status := C.GetJavaVM(env.value(), &jVM); status != 0 {
		return fmt.Errorf("couldn't get Java VM from the (Java) environment")
	}
	return nil
}

//export Java_io_v_util_NativeCallback_nativeOnSuccess
func Java_io_v_util_NativeCallback_nativeOnSuccess(jenv *C.JNIEnv, jNativeCallback C.jobject, goSuccessRef C.jlong, jResultObj C.jobject) {
	jResult := Object(uintptr(unsafe.Pointer(jResultObj)))
	(*(*func(Object))(GoRefValue(Ref(goSuccessRef))))(jResult)
}

//export Java_io_v_util_NativeCallback_nativeOnFailure
func Java_io_v_util_NativeCallback_nativeOnFailure(jenv *C.JNIEnv, jNativeCallback C.jobject, goFailureRef C.jlong, jVException C.jobject) {
	env := Env(uintptr(unsafe.Pointer(jenv)))
	err := GoError(env, Object(uintptr(unsafe.Pointer(jVException))))
	(*(*func(error))(GoRefValue(Ref(goFailureRef))))(err)
}

//export Java_io_v_util_NativeCallback_nativeFinalize
func Java_io_v_util_NativeCallback_nativeFinalize(jenv *C.JNIEnv, jNativeCallback C.jobject, goSuccessRef C.jlong, goFailureRef C.jlong) {
	GoDecRef(Ref(goSuccessRef))
	GoDecRef(Ref(goFailureRef))
}
