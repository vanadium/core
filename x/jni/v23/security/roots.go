// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package security

import (
	"log"
	"runtime"

	"v.io/v23/security"
	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

// JavaBlessingRoots creates an instance of Java BlessingRoots that uses the provided Go
// BlessingRoots as its underlying implementation.
func JavaBlessingRoots(env jutil.Env, roots security.BlessingRoots) (jutil.Object, error) {
	ref := jutil.GoNewRef(&roots) // Un-refed when the Java BlessingRootsImpl is finalized.
	jRoots, err := jutil.NewObject(env, jBlessingRootsImplClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jRoots, nil
}

// GoBlessingRoots creates an instance of security.BlessingRoots that uses the
// provided Java BlessingRoots as its underlying implementation.
func GoBlessingRoots(env jutil.Env, jBlessingRoots jutil.Object) (security.BlessingRoots, error) {
	if jBlessingRoots.IsNull() {
		return nil, nil
	}
	if jutil.IsInstanceOf(env, jBlessingRoots, jBlessingRootsImplClass) {
		// Called with our implementation of BlessingRoots, which maintains a Go reference - use it.
		ref, err := jutil.CallLongMethod(env, jBlessingRoots, "nativeRef", nil)
		if err != nil {
			return nil, err
		}
		return (*(*security.BlessingRoots)(jutil.GoRefValue(jutil.Ref(ref)))), nil
	}
	// Reference Java BlessingRoots; it will be de-referenced when the Go
	// BlessingRoots created below is garbage-collected (through the finalizer
	// callback we setup just below).
	jBlessingRoots = jutil.NewGlobalRef(env, jBlessingRoots)
	r := &blessingRoots{
		jBlessingRoots: jBlessingRoots,
	}
	runtime.SetFinalizer(r, func(r *blessingRoots) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jutil.DeleteGlobalRef(env, r.jBlessingRoots)
	})
	return r, nil
}

type blessingRoots struct {
	jBlessingRoots jutil.Object
}

func (r *blessingRoots) Add(root []byte, pattern security.BlessingPattern) error {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jRoot, err := JavaPublicKeyFromDER(env, root)
	if err != nil {
		return err
	}
	jPattern, err := JavaBlessingPattern(env, pattern)
	if err != nil {
		return err
	}
	return jutil.CallVoidMethod(env, r.jBlessingRoots, "add", []jutil.Sign{publicKeySign, blessingPatternSign}, jRoot, jPattern)
}

func (r *blessingRoots) Recognized(root []byte, blessing string) error {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jRoot, err := JavaPublicKeyFromDER(env, root)
	if err != nil {
		return err
	}
	return jutil.CallVoidMethod(env, r.jBlessingRoots, "recognized", []jutil.Sign{publicKeySign, jutil.StringSign}, jRoot, blessing)
}

func (r *blessingRoots) DebugString() string {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	ret, err := jutil.CallStringMethod(env, r.jBlessingRoots, "debugString", nil)
	if err != nil {
		log.Printf("Couldn't get Java DebugString: %v", err)
		return ""
	}
	return ret
}

func (r *blessingRoots) Dump() map[security.BlessingPattern][]security.PublicKey {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	ret, err := jutil.CallMultimapMethod(env, r.jBlessingRoots, "dump", []jutil.Sign{})
	if err != nil {
		log.Printf("Couldn't get Java Dump: %v", err)
		return nil
	}
	result := make(map[security.BlessingPattern][]security.PublicKey)
	for jPattern, jKeys := range ret {
		pattern, err := GoBlessingPattern(env, jPattern)
		if err != nil {
			log.Printf("Couldn't convert Java BlessingPattern: %v", err)
			return nil
		}
		var entry []security.PublicKey
		for _, jKey := range jKeys {
			key, err := GoPublicKey(env, jKey)
			if err != nil {
				log.Printf("Couldn't convert Java PublicKey: %v", err)
				return nil
			}
			entry = append(entry, key)
		}
		result[pattern] = entry
	}
	return result
}
