// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

/*
import (
	"reflect"
	"sync"
)

// perfCacheT is a performance enhancing series of caches and is not required for
// correctness. It uses sync.Map to allow for scaling across multiple cores.
type perfCacheT struct {
	reflectCache      *sync.Map // map[reflect.Type]cachedReflectInfo
	validityCache     *sync.Map // map[string]error // keyed by Type.unique
	optionalTypeCache *sync.Map // map[*Type]*Type
	zeroValueCache    *sync.Map // map[reflect.Type]reflect.Value
}

var perfCache = perfCacheT{
	reflectCache:      &sync.Map{}, // map[reflect.Type]cachedReflectInfo{},
	validityCache:     &sync.Map{}, // map[string]error{},
	optionalTypeCache: &sync.Map{}, // map[*Type]*Type{},
	zeroValueCache:    &sync.Map{}, // map[reflect.Type]reflect.Value{},
}

type cachedReflectInfo struct {
	hasZeroer       bool
	ptrHasZeroer    bool
	ptrHasVDLWriter bool
	ni              *nativeInfo
}

func (pc *perfCacheT) lookupReflectInfo(tt reflect.Type) cachedReflectInfo {
	val, ok := pc.reflectCache.Load(tt)
	if ok {
		return val.(cachedReflectInfo)
	}
	// There is a race here since it's possible for multiple threads
	// to be simulatenaously computing this data.
	ci := cachedReflectInfo{
		hasZeroer:       tt.Implements(rtIsZeroer),
		ptrHasVDLWriter: reflect.PtrTo(tt).Implements(rtVDLWriter),
		ni:              nativeInfoFromNative(tt),
	}
	if !ci.hasZeroer {
		ci.ptrHasZeroer = reflect.PtrTo(tt).Implements(rtIsZeroer)
	}
	pc.reflectCache.Store(tt, ci)
	return ci
}

func (pc *perfCacheT) validType(unique string) (bool, error) {
	val, ok := pc.validityCache.Load(unique)
	if ok {
		err, _ := val.(error)
		return true, err
	}
	return false, nil
}

func (pc *perfCacheT) saveTypeValidity(unique string, err error) {
	pc.validityCache.Store(unique, err)
}

func (pc *perfCacheT) optionalTypeFor(t *Type) *Type {
	if v, ok := pc.optionalTypeCache.Load(t); ok {
		return v.(*Type)
	}
	return nil
}

func (pc *perfCacheT) setOptionalType(elem, opt *Type) {
	pc.optionalTypeCache.Store(elem, opt)
}

// DOESN"T WORK YET.
func (pc *perfCacheT) zeroValueFor(t reflect.Type) (reflect.Value, bool) {
	if v, ok := pc.zeroValueCache.Load(t); ok {
		return v.(reflect.Value), ok
	}
	return reflect.Value{}, false
}

func (pc *perfCacheT) setZeroValueFor(t reflect.Type, v reflect.Value) {
	ptrTo := reflect.New(t)
	reflect.Indirect(ptrTo).Set(v)
	pc.zeroValueCache.Store(t, ptrTo.Elem())
}
*/
