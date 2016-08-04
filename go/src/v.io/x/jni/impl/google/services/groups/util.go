// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package groups

import (
	jutil "v.io/x/jni/util"
)

// GoStorageEngine converts the provided Java GroupServer.StorageEngine
// enum object into a Go storage engine string.
func GoStorageEngine(env jutil.Env, jEngine jutil.Object) (string, error) {
	if jEngine.IsNull() {
		return "leveldb", nil
	}
	return jutil.CallStringMethod(env, jEngine, "getValue", []jutil.Sign{})
}
