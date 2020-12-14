// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"io"
	"sync"
	"unsafe"

	"v.io/v23/vdl"
)

// Encode writes the value v and returns the encoded bytes.  The semantics of
// value encoding are described by Encoder.Encode.
//
// This is a "single-shot" encoding; full type information is always included in
// the returned encoding, as if a new encoder were used for each call.
func Encode(v interface{}) ([]byte, error) {
	return VersionedEncode(DefaultVersion, v)
}

// VersionedEncode performs single-shot encoding to a specific version of VOM
func VersionedEncode(version Version, v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := NewVersionedEncoder(version, &buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode reads the value from the given data, and stores it in value v.  The
// semantics of value decoding are described by Decoder.Decode.
//
// This is a "single-shot" decoding; the data must have been encoded by a call
// to vom.Encode.
func Decode(data []byte, v interface{}) error {
	// The implementation below corresponds (logically) to the following:
	//   return NewDecoder(bytes.NewReader(data)).Decode(valptr)
	//
	// However decoding type messages is expensive, so we cache the
	// results of typeDecoders to skip the decoding in the common case.

	bufs := reusableBuffersPool.Get().(*reusableBuffers)
	defer reusableBuffersPool.Put(bufs)
	bufs.dec.buf.SetBytes(data)
	buf := bufs.dec.buf

	keyBytes, err := computeTypeDecoderCacheKey(buf)
	if err != nil {
		return err
	}
	var key string
	if keyBytes != nil {
		// Avoid unnecessary string copy to convert []byte to string
		// just use for lookups here.
		key = *(*string)(unsafe.Pointer(&keyBytes))
	}
	typeInfo, ok := singleShotTypeDecoderCache.lookup(key)
	cacheMiss := false
	var typeDec *TypeDecoder
	if !ok {
		// Cache miss; start decoding at the beginning of all type messages with a
		// new TypeDecoder.
		cacheMiss = true
		buf.beg = 0
		typeDec = newTypeDecoderInternal(bufs.dec)
	} else {
		// Cache hit; the buf is already positioned on the message id of the value,
		// so we can just continue decoding from there.
		buf.version = Version(data[0])
		typeDec = newDerivedTypeDecoderInternal(bufs.dec, typeInfo.idToType, typeInfo.idToWire)
	}
	// Decode the value message.
	dec := &Decoder{decoder81{
		buf:     buf,
		typeDec: typeDec,
	}}
	if err := dec.Decode(v); err != nil {
		return err
	}
	// Populate the typeDecoder cache for future re-use.
	if cacheMiss {
		singleShotTypeDecoderCache.insert(key, typeDec.idToType, typeDec.idToWire)
	}
	return nil
}

// singleShotTypeDecoderCache is a global cache of TypeDecoders keyed by the
// bytes of the sequence of type messages before the value message.  A sequence
// of type messages that is byte-equal doesn't guarantee the same types in the
// general case, since type ids are scoped to a single Encoder/Decoder stream;
// different streams may represent different types using the same type ids.
// However the single-shot vom.Encode is guaranteed to generate the same bytes
// for a given vom version.
//
// TODO(toddw): This cache grows without bounds, use a fixed-size cache instead.

type typeInfo struct {
	idToType map[TypeId]*vdl.Type
	idToWire map[TypeId]wireType
}

var singleShotTypeDecoderCache = typeDecoderCache{
	decoders: make(map[string]typeInfo),
}

type typeDecoderCache struct {
	sync.RWMutex
	decoders map[string]typeInfo
}

func (x *typeDecoderCache) insert(key string,
	idToType map[TypeId]*vdl.Type,
	idToWire map[TypeId]wireType) {
	x.Lock()
	defer x.Unlock()
	if _, ok := x.decoders[key]; ok {
		// There was a race between concurrent calls to vom.Decode, and another
		// goroutine already populated the cache.  It doesn't matter which one we
		// use, so there's nothing more to do.
		return
	}
	x.decoders[key] = typeInfo{
		idToType: idToType,
		idToWire: idToWire,
	}
}

func (x *typeDecoderCache) lookup(key string) (typeInfo, bool) {
	x.RLock()
	ti, ok := x.decoders[key]
	x.RUnlock()
	return ti, ok
}

// computeTypeDecoderCacheKey computes the cache key for the typeDecoderCache,
// by returning the bytes of all type messages.  Upon return, the read position
// of b is guaranteed to be on the first byte of the value message.
func computeTypeDecoderCacheKey(b *decbuf) ([]byte, error) {
	if b.end == 0 {
		return nil, io.EOF
	}
	if version := Version(b.buf[0]); !isAllowedVersion(version) {
		return nil, errBadVersionByte(version)
	}
	b.beg++
	// Walk through bytes until we get to a value message.
	for {
		if b.end < b.beg {
			return nil, errIndexOutOfRange
		}
		// Handle incomplete types.
		switch ok, err := binaryDecodeControlOnly(b, WireCtrlTypeIncomplete); {
		case err != nil:
			return nil, err
		case ok:
			continue
		}
		// Handle the next message id.
		switch id, byteLen, err := binaryPeekInt(b); {
		case err != nil:
			return nil, err
		case id > 0:
			// This is a value message.  The bytes read so far include the version
			// byte and all type messages; use all of these bytes as the cache key.
			//
			// TODO(toddw): Take a fingerprint of these bytes to reduce memory usage.
			return b.buf[:b.beg], nil
		case id < 0:
			// This is a type message.  Skip the bytes for the id or control code, and
			// decode the message length (which always exists for wireType), and skip
			// those bytes too to move to the next message.
			b.beg += byteLen
			msgLen, err := binaryDecodeLen(b)
			if err != nil {
				return nil, err
			}
			b.beg += msgLen
		default:
			return nil, errDecodeZeroTypeID
		}
	}
}
