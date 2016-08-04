// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java

package ble

import (
	jutil "v.io/x/jni/util"
)

// Init initializes the JNI code with the given Java environment. This method
// must be called from the main Java thread.
// TODO(mattr): Why must this be called from the main thread?
func Init(env jutil.Env) error {
	return nil
}
