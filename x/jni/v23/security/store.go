// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package security

import (
	"log"
	"runtime"
	"time"

	"v.io/v23/security"
	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

// JavaBlessingStore creates an instance of Java BlessingStore that uses the provided Go
// BlessingStore as its underlying implementation.
func JavaBlessingStore(env jutil.Env, store security.BlessingStore) (jutil.Object, error) {
	ref := jutil.GoNewRef(&store) // Un-refed when the Java BlessingStoreImpl is finalized.
	jObj, err := jutil.NewObject(env, jBlessingStoreImplClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jObj, nil
}

// GoBlessingStore creates an instance of security.BlessingStore that uses the
// provided Java BlessingStore as its underlying implementation.
func GoBlessingStore(env jutil.Env, jBlessingStore jutil.Object) (security.BlessingStore, error) {
	if jBlessingStore.IsNull() {
		return nil, nil
	}
	if jutil.IsInstanceOf(env, jBlessingStore, jBlessingStoreImplClass) {
		// Called with our implementation of BlessingStore, which maintains a Go reference - use it.
		ref, err := jutil.JLongField(env, jBlessingStore, "nativeRef")
		if err != nil {
			return nil, err
		}
		return (*(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(ref)))), nil
	}
	// Reference Java BlessingStore; it will be de-referenced when the Go
	// BlessingStore created below is garbage-collected (through the finalizer
	// callback we setup just below).
	jBlessingStore = jutil.NewGlobalRef(env, jBlessingStore)
	s := &blessingStore{
		jBlessingStore: jBlessingStore,
	}
	runtime.SetFinalizer(s, func(s *blessingStore) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jutil.DeleteGlobalRef(env, s.jBlessingStore)
	})
	return s, nil
}

type blessingStore struct {
	jBlessingStore jutil.Object
}

func (s *blessingStore) Set(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, error) {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		return security.Blessings{}, err
	}
	jForPeers, err := JavaBlessingPattern(env, forPeers)
	if err != nil {
		return security.Blessings{}, err
	}
	jOldBlessings, err := jutil.CallObjectMethod(env, s.jBlessingStore, "set", []jutil.Sign{blessingsSign, blessingPatternSign}, blessingsSign, jBlessings, jForPeers)
	if err != nil {
		return security.Blessings{}, err
	}
	return GoBlessings(env, jOldBlessings)
}

func (s *blessingStore) ForPeer(peerBlessings ...string) security.Blessings {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jBlessings, err := jutil.CallObjectMethod(env, s.jBlessingStore, "forPeer", []jutil.Sign{jutil.ArraySign(jutil.StringSign)}, blessingsSign, peerBlessings)
	if err != nil {
		log.Printf("Couldn't call Java forPeer method: %v", err)
		return security.Blessings{}
	}
	blessings, err := GoBlessings(env, jBlessings)
	if err != nil {
		log.Printf("Couldn't convert Java Blessings into Go: %v", err)
		return security.Blessings{}
	}
	return blessings
}

func (s *blessingStore) SetDefault(blessings security.Blessings) error {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		return err
	}
	return jutil.CallVoidMethod(env, s.jBlessingStore, "setDefaultBlessings", []jutil.Sign{blessingsSign}, jBlessings)
}

func (s *blessingStore) Default() (security.Blessings, <-chan struct{}) {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	// TODO(ashankar,spetrovic): Figure out notification API in Java
	jBlessings, err := jutil.CallObjectMethod(env, s.jBlessingStore, "defaultBlessings", nil, blessingsSign)
	if err != nil {
		log.Printf("Couldn't call Java defaultBlessings method: %v", err)
		return security.Blessings{}, nil
	}
	blessings, err := GoBlessings(env, jBlessings)
	if err != nil {
		log.Printf("Couldn't convert Java Blessings to Go Blessings: %v", err)
		return security.Blessings{}, nil
	}
	return blessings, nil
}

func (s *blessingStore) PublicKey() security.PublicKey {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jPublicKey, err := jutil.CallObjectMethod(env, s.jBlessingStore, "publicKey", nil, publicKeySign)
	if err != nil {
		log.Printf("Couldn't get Java public key: %v", err)
		return nil
	}
	publicKey, err := GoPublicKey(env, jPublicKey)
	if err != nil {
		log.Printf("Couldn't convert Java ECPublicKey to Go PublicKey: %v", err)
		return nil
	}
	return publicKey
}

func (s *blessingStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jBlessingsMap, err := jutil.CallObjectMethod(env, s.jBlessingStore, "peerBlessings", nil, jutil.MapSign)
	if err != nil {
		log.Printf("Couldn't get Java peer blessings: %v", err)
		return nil
	}
	bmap, err := jutil.GoObjectMap(env, jBlessingsMap)
	if err != nil {
		log.Printf("Couldn't convert Java object map into a Go object map: %v", err)
		return nil
	}
	ret := make(map[security.BlessingPattern]security.Blessings)
	for jPattern, jBlessings := range bmap {
		pattern, err := GoBlessingPattern(env, jPattern)
		if err != nil {
			log.Printf("Couldn't convert Java pattern into Go: %v", err)
			return nil
		}
		blessings, err := GoBlessings(env, jBlessings)
		if err != nil {
			log.Printf("Couldn't convert Java blessings into Go: %v", err)
			return nil
		}
		ret[pattern] = blessings
	}
	return ret
}

func (s *blessingStore) CacheDischarge(discharge security.Discharge, caveat security.Caveat, impetus security.DischargeImpetus) {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jDischarge, err := JavaDischarge(env, discharge)
	if err != nil {
		log.Printf("Couldn't get Java discharge: %v", err)
		return
	}
	jCaveat, err := JavaCaveat(env, caveat)
	if err != nil {
		log.Printf("Couldn't get Java caveat: %v", err)
		return
	}
	jImpetus, err := jutil.JVomCopy(env, impetus, jDischargeImpetusClass)
	if err != nil {
		log.Printf("Couldn't get Java DischargeImpetus: %v", err)
		return
	}
	err = jutil.CallVoidMethod(env, s.jBlessingStore, "cacheDischarge", []jutil.Sign{dischargeSign, caveatSign, dischargeImpetusSign}, jDischarge, jCaveat, jImpetus)
	if err != nil {
		log.Printf("Couldn't call cacheDischarge: %v", err)
	}
}

func (s *blessingStore) ClearDischarges(discharges ...security.Discharge) {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jDischarges := make([]jutil.Object, len(discharges))
	for i := 0; i < len(discharges); i++ {
		jDischarge, err := JavaDischarge(env, discharges[i])
		if err != nil {
			log.Printf("Couldn't get Java discharge: %v", err)
			return
		}
		jDischarges[i] = jDischarge
	}
	jDischargeList, err := jutil.JObjectList(env, jDischarges, jDischargeClass)
	if err != nil {
		log.Printf("Couldn't get Java discharge list: %v", err)
		return
	}
	err = jutil.CallVoidMethod(env, s.jBlessingStore, "clearDischarges", []jutil.Sign{jutil.ListSign}, jDischargeList)
	if err != nil {
		log.Printf("Couldn't call Java clearDischarges method: %v", err)
	}
}

// TODO(sjr): support cachedTime in Java
func (s *blessingStore) Discharge(caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, time.Time) {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jCaveat, err := JavaCaveat(env, caveat)
	if err != nil {
		log.Printf("Couldn't get Java caveat: %v", err)
		return security.Discharge{}, time.Time{}
	}
	jImpetus, err := jutil.JVomCopy(env, impetus, jDischargeImpetusClass)
	if err != nil {
		log.Printf("Couldn't get Java DischargeImpetus: %v", err)
		return security.Discharge{}, time.Time{}
	}
	jDischarge, err := jutil.CallObjectMethod(env, s.jBlessingStore, "discharge", []jutil.Sign{caveatSign, dischargeImpetusSign}, dischargeSign, jCaveat, jImpetus)
	if err != nil {
		log.Printf("Couldn't call Java discharge method: %v", err)
		return security.Discharge{}, time.Time{}
	}
	discharge, err := GoDischarge(env, jDischarge)
	if err != nil {
		log.Printf("Couldn't convert Java discharge to Go: %v", err)
		return security.Discharge{}, time.Time{}
	}
	return discharge, time.Time{}
}

func (r *blessingStore) DebugString() string {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	result, err := jutil.CallStringMethod(env, r.jBlessingStore, "debugString", nil)
	if err != nil {
		log.Printf("Couldn't call Java debugString: %v", err)
		return ""
	}
	return result
}
