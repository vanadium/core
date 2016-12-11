// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package naming

import (
	"v.io/v23/naming"

	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

// JavaEndpoint converts the provided Go Endpoint to a Java Endpoint.
func JavaEndpoint(env jutil.Env, endpoint naming.Endpoint) (jutil.Object, error) {
	return jutil.CallStaticObjectMethod(env, jEndpointImplClass, "fromString", []jutil.Sign{jutil.StringSign}, endpointSign, endpoint.String())
}
