// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package naming

import (
	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

var (
	endpointSign = jutil.ClassSign("io.v.v23.naming.Endpoint")

	// Global reference for io.v.impl.google.naming.EndpointImpl.
	jEndpointImplClass jutil.Class
)

func Init(env jutil.Env) error {
	// Cache global references to all Java classes used by the package.  This is
	// necessary because JNI gets access to the class loader only in the system
	// thread, so we aren't able to invoke FindClass in other threads.
	var err error
	jEndpointImplClass, err = jutil.JFindClass(env, "io/v/impl/google/naming/EndpointImpl")
	if err != nil {
		return err
	}
	return nil
}
