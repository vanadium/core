package vdl

import (
	"fmt"
	"reflect"
	"sync"
)

type perfReflectCacheT struct {
	sync.RWMutex
	rtmap map[reflect.Type]perfReflectInfo
}

type perfReflectInfo struct {
	keyType         reflect.Type
	fieldIndexMap   map[string]int
	implementsIfc   implementsBitMasks
	implementsIsSet implementsBitMasks
	nativeInfo      *nativeInfo
	nativeInfoIsSet bool
}

var perfReflectCache = &perfReflectCacheT{
	rtmap: make(map[reflect.Type]perfReflectInfo),
}

type implementsBitMasks uint16

const (
	rtIsZeroerBitMask implementsBitMasks = 1 << iota
	rtVDLWriterBitMask
	rtErrorBitMask

	rtIsZeroerPtrToBitMask implementsBitMasks = 8 << iota
	rtVDLWriterPtrToBitMask
)

// getPerReflectInfo returns the currently cached info, however, that may be
// empty or incomplete and the per-field methods used to determine if a
// specified field is set or not and to fill it in if not set. This API is used
// to minimize the number of lock aquistions required for the common case where
// the data is already available and to allow data to be filled in incrementally
// as required, since the common case is that most of the fields are not
// used for a given type.
//
// For example, the following only requires one lock operation for the case
// where both required fields are already available.
//
//   pri := perfReflectCache.perfReflectInfo(rt)
//     ....
//   idxMap := perfReflectCache.fieldIndexMap(pri, rt)
//     ....
//   hasZeroer := perfReflectCache.implements(pri, rt, rtIsZeroerBitMask)
//     ....
//   hasPtrZeroer := perfReflectCache.implements(pri, rt, rtIsZeroerBitMask|rtIsPtrToMask)
func (prc *perfReflectCacheT) perfReflectInfo(rt reflect.Type) perfReflectInfo {
	prc.RLock()
	r := prc.rtmap[rt]
	prc.RUnlock()
	return r
}

// fieldIndexMap returns a map the index of the struct field in rt with the given
// name.
//
// This function is purely a performance optimization; the current
// implementation of reflect.Type.Field(index) causes an allocation, which is
// avoided in the common case by caching the result.
//
// REQUIRES: rt.Kind() == reflect.Struct
func (prc *perfReflectCacheT) fieldIndexMap(pri perfReflectInfo, rt reflect.Type) map[string]int {
	if pri.keyType == rt && pri.fieldIndexMap != nil {
		return pri.fieldIndexMap
	}
	return prc.cacheFieldIndexMap(rt)
}

//go:noinline
func (prc *perfReflectCacheT) cacheFieldIndexMap(rt reflect.Type) map[string]int {
	prc.Lock()
	defer prc.Unlock()
	// benign race between read and write locks.
	pri, ok := prc.rtmap[rt]
	if ok && pri.fieldIndexMap != nil {
		return pri.fieldIndexMap
	}
	pri.fieldIndexMap = createFieldIndexMap(rt)
	pri.keyType = rt
	prc.rtmap[rt] = pri
	return pri.fieldIndexMap
}

func createFieldIndexMap(rt reflect.Type) map[string]int {
	if numField := rt.NumField(); numField > 0 {
		m := make(map[string]int, numField)
		for i := 0; i < numField; i++ {
			m[rt.Field(i).Name] = i
		}
		return m
	}
	return nil
}

func (prc *perfReflectCacheT) implementsBuiltinInterface(pri perfReflectInfo, rt reflect.Type, mask implementsBitMasks) bool {
	if pri.keyType == rt && (pri.implementsIsSet&mask) != 0 {
		return (pri.implementsIfc & mask) != 0
	}
	return prc.cacheImplementsBuiltinInterface(rt, mask)
}

//go:noinline
func (prc *perfReflectCacheT) cacheImplementsBuiltinInterface(rt reflect.Type, mask implementsBitMasks) bool {
	prc.Lock()
	defer prc.Unlock()
	// benign race between read and write locks.
	pri := prc.rtmap[rt]
	if (pri.implementsIsSet & mask) != 0 {
		return (pri.implementsIfc & mask) != 0
	}
	var target reflect.Type
	switch mask {
	case rtIsZeroerBitMask, rtIsZeroerPtrToBitMask:
		target = rtIsZeroer
	case rtVDLWriterBitMask, rtVDLWriterPtrToBitMask:
		target = rtVDLWriter
	case rtErrorBitMask:
		target = rtError
	default:
		panic(fmt.Sprintf("invalid bit mask for interface implementation test: %b", mask))
	}
	result := false
	if mask >= rtIsZeroerPtrToBitMask { // first PtrTo type.
		result = reflect.PtrTo(rt).Implements(target)
	} else {
		result = rt.Implements(target)
	}
	if result {
		pri.implementsIfc = (pri.implementsIfc & ^mask) | mask
	}
	pri.implementsIsSet = (pri.implementsIsSet & ^mask) | mask
	pri.keyType = rt
	prc.rtmap[rt] = pri
	return result
}

func (prc *perfReflectCacheT) nativeInfo(pri perfReflectInfo, rt reflect.Type) *nativeInfo {
	if pri.nativeInfoIsSet {
		return pri.nativeInfo
	}
	return prc.cacheNativeInfo(rt)
}

//go:noinline
func (prc *perfReflectCacheT) cacheNativeInfo(rt reflect.Type) *nativeInfo {
	prc.Lock()
	defer prc.Unlock()
	// benign race between read and write locks.
	pri := prc.rtmap[rt]
	if pri.nativeInfoIsSet {
		return pri.nativeInfo
	}
	pri.nativeInfo = nativeInfoFromNative(rt)
	pri.nativeInfoIsSet = true
	pri.keyType = rt
	prc.rtmap[rt] = pri
	return pri.nativeInfo
}
