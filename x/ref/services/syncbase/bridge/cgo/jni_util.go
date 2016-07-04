// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android
// +build cgo

package main

import (
	"fmt"
	"runtime"
	"unsafe"
)

// #include <stdlib.h>
// #include "jni_wrapper.h"
// #include "lib.h"
import "C"

func jGetMethodID(env *C.JNIEnv, cls C.jclass, name, sig string) C.jmethodID {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	cSig := C.CString(sig)
	defer C.free(unsafe.Pointer(cSig))

	method := C.GetMethodID(env, cls, cName, cSig)
	if method == nil {
		panic(fmt.Sprintf("couldn't get method %q with signature %s", name, sig))
	}

	// Note: the validity of the method is bounded by the lifetime of the
	// ClassLoader that did the loading of the class.
	return method
}

func jGetFieldID(env *C.JNIEnv, cls C.jclass, name, sig string) C.jfieldID {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	cSig := C.CString(sig)
	defer C.free(unsafe.Pointer(cSig))

	field := C.GetFieldID(env, cls, cName, cSig)
	if field == nil {
		panic(fmt.Sprintf("couldn't get field %q with signature %s", name, sig))
	}

	// Note: the validity of the field is bounded by the lifetime of the
	// ClassLoader that did the loading of the class.
	return field
}

func jGetStaticFieldID(env *C.JNIEnv, cls C.jclass, name, sig string) C.jfieldID {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	cSig := C.CString(sig)
	defer C.free(unsafe.Pointer(cSig))

	field := C.GetStaticFieldID(env, cls, cName, cSig)
	if field == nil {
		panic(fmt.Sprintf("couldn't get field %q with signature %s", name, sig))
	}

	// Note: the validity of the field is bounded by the lifetime of the
	// ClassLoader that did the loading of the class.
	return field
}

// The function from below was hoisted from jni/util/util.go and adapted to not
// use custom types.

// GetEnv returns the Java environment for the running thread, creating a new
// one if it doesn't already exist.  This method also returns a function which
// must be invoked when the returned environment is no longer needed. The
// returned environment can only be used by the thread that invoked this method,
// and the function must be invoked by the same thread as well.
func getEnv() (*C.JNIEnv, func()) {
	// Lock the goroutine to the current OS thread.  This is necessary as
	// *C.JNIEnv must not be shared across threads.  The scenario that can
	// break this requirement is:
	//   - goroutine A executing on thread X, obtains a *C.JNIEnv pointer P.
	//   - goroutine A gets re-scheduled on thread Y, maintaining the P.
	//   - goroutine B starts executing on thread X, obtaining pointer P.
	//
	// By locking the goroutines to their OS thread while they hold the
	// pointer to *C.JNIEnv, the above scenario can never occur.
	runtime.LockOSThread()
	var env *C.JNIEnv
	if C.GetEnv(jVM, &env, C.JNI_VERSION_1_6) != C.JNI_OK {
		// Couldn't get env; attach the thread.  Note that we never
		// detach the thread, so the next call to GetEnv on this thread
		// will succeed. We also don't have to worry about calling
		// DetachCurrentThread before the thread exits.
		C.AttachCurrentThreadAsDaemon(jVM, &env, nil)
	}
	// GetEnv is called by Go code that wishes to call Java methods. In
	// this case, JNI cannot automatically free unused local references.
	// We must do it manually by pushing a new local reference frame. The
	// frame will be popped in the env's cleanup function below, at which
	// point JNI will free the unused references.
	// http://developer.android.com/training/articles/perf-jni.html states
	// that the JNI implementation is only required to provide a local
	// reference table with a capacity of 16, so here we provide a table of
	// that size.
	localRefCapacity := 16
	if newCapacity := C.PushLocalFrame(env, C.jint(localRefCapacity)); newCapacity < 0 {
		panic("PushLocalFrame(" + string(localRefCapacity) + ") returned < 0 (was " + string(newCapacity) + ")")
	}
	return env, func() {
		C.PopLocalFrame(env, nil)
		runtime.UnlockOSThread()
	}
}
