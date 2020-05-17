// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java

package jni

import (
	"unsafe"

	jutil "v.io/x/jni/util"
	"v.io/x/lib/vlog"
)

// #include "jni.h"
import "C"

//export Java_io_v_v23_V_nativeInitGlobalJava
func Java_io_v_v23_V_nativeInitGlobalJava(jenv *C.JNIEnv, jVClass C.jclass, jOptions C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jOpts := jutil.Object(uintptr(unsafe.Pointer(jOptions)))

	// Setup logging.
	dir, toStderr, level, vmodule, err := loggingOpts(env, jOpts)
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	vlog.Log.Configure(vlog.OverridePriorConfiguration(true), dir, toStderr, level, vmodule)
}
