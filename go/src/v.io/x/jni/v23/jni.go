// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package v23

import (
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
	ji18n "v.io/x/jni/v23/i18n"
	jnaming "v.io/x/jni/v23/naming"
	jsecurity "v.io/x/jni/v23/security"
	jaccess "v.io/x/jni/v23/security/access"
	jgroups "v.io/x/jni/v23/services/groups"
)

// #include "jni.h"
import "C"

// Init initializes the JNI code with the given Java environment.  This method
// must be invoked before any other method in this package and must be called
// from the main Java thread (e.g., On_Load()).
func Init(env jutil.Env) error {
	if err := jcontext.Init(env); err != nil {
		return err
	}
	if err := ji18n.Init(env); err != nil {
		return err
	}
	if err := jnaming.Init(env); err != nil {
		return err
	}
	if err := jsecurity.Init(env); err != nil {
		return err
	}
	if err := jaccess.Init(env); err != nil {
		return err
	}
	if err := jgroups.Init(env); err != nil {
		return err
	}

	return nil
}
