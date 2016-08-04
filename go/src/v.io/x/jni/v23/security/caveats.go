// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package security

import (
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/uniqueid"

	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
)

// #include "jni.h"
import "C"

var systemCaveats = map[uniqueid.Id]bool{
	security.ConstCaveat.Id:               true,
	security.ExpiryCaveat.Id:              true,
	security.MethodCaveat.Id:              true,
	security.PeerBlessingsCaveat.Id:       true,
	security.PublicKeyThirdPartyCaveat.Id: true,
	access.AccessTagCaveat.Id:             true,
}

func caveatValidator(context *context.T, call security.Call, sets [][]security.Caveat) []error {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jContext, err := jcontext.JavaContext(env, context, nil)
	if err != nil {
		return errors(err, len(sets))
	}
	jCall, err := JavaCall(env, call)
	if err != nil {
		return errors(err, len(sets))
	}
	ret := make([]error, len(sets))
	for i, set := range sets {
		for _, caveat := range set {
			// NOTE(spetrovic): We validate system caveats in Go as it is significantly faster.
			if _, ok := systemCaveats[caveat.Id]; ok {
				if err := caveat.Validate(context, call); err != nil {
					ret[i] = err
					break
				}
			} else {
				jCaveat, err := JavaCaveat(env, caveat)
				if err != nil {
					ret[i] = err
					break
				}
				if err := jutil.CallStaticVoidMethod(env, jCaveatRegistryClass, "validate", []jutil.Sign{contextSign, callSign, caveatSign}, jContext, jCall, jCaveat); err != nil {
					ret[i] = err
					break
				}
			}
		}
	}
	return ret
}

func errors(err error, len int) []error {
	ret := make([]error, len)
	for i := 0; i < len; i++ {
		ret[i] = err
	}
	return ret
}
