// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"errors"
	"fmt"

	"v.io/v23/vdl"
)

var (
	errIndexOutOfRange  = errors.New("vom: index out of range")
	errDecodeZeroTypeID = errors.New("vom: zero type id")
)

func (d *decoder81) decodeTypeDefs() error {
	for {
		typeNext, err := d.typeIsNext()
		if err != nil {
			return err
		}
		if !typeNext {
			return nil
		}
		if err := d.typeDec.readSingleType(); err != nil {
			return err
		}
	}
}

// peekValueByteLen returns the byte length of the next value.
func (d *decoder81) peekValueByteLen(tt *vdl.Type) (int, error) {
	if hasChunkLen(tt) {
		// Use the explicit message length.
		return d.buf.lim, nil
	}
	// No explicit message length, but the length can be computed.
	switch {
	case tt.Kind() == vdl.Array && tt.IsBytes():
		// Byte arrays are exactly their length and encoded with 1-byte header.
		return tt.Len() + 1, nil
	case tt.Kind() == vdl.String || tt.IsBytes():
		// Strings and byte lists are encoded with a length header.
		strlen, bytelen, err := binaryPeekUint(d.buf)
		switch {
		case err != nil:
			return 0, err
		case strlen > maxBinaryMsgLen:
			return 0, errMsgLen(maxBinaryMsgLen)
		}
		return int(strlen) + bytelen, nil
	default:
		// Must be a primitive, which is encoded as an underlying uint.
		return binaryPeekUintByteLen(d.buf)
	}
}

func (d *decoder81) decodeRaw(tt *vdl.Type, valLen int, raw *RawBytes) error {
	raw.Version = d.buf.version
	raw.Type = tt
	raw.Data = make([]byte, valLen)
	if err := d.buf.ReadIntoBuf(raw.Data); err != nil {
		return err
	}
	refTypeLen := len(d.refTypes.tids)
	if cap(raw.RefTypes) >= refTypeLen {
		raw.RefTypes = raw.RefTypes[:refTypeLen]
	} else {
		raw.RefTypes = make([]*vdl.Type, refTypeLen)
	}
	for i, tid := range d.refTypes.tids {
		var err error
		if raw.RefTypes[i], err = d.typeDec.lookupType(tid); err != nil {
			return err
		}
	}
	raw.AnyLengths = d.refAnyLens.lens
	return nil
}

func (d *decoder81) readAnyHeader() (*vdl.Type, int, error) {
	// Handle WireCtrlNil.
	switch ok, err := binaryDecodeControlOnly(d.buf, WireCtrlNil); {
	case err != nil:
		return nil, 0, err
	case ok:
		return nil, 0, nil // nil any
	}
	// Read the index of the referenced type id.
	typeIndex, err := binaryDecodeUint(d.buf)
	if err != nil {
		return nil, 0, err
	}
	var tid TypeId
	if d.buf.version == Version80 {
		tid = TypeId(typeIndex)
	} else if tid, err = d.refTypes.ReferencedTypeID(typeIndex); err != nil {
		return nil, 0, err
	}
	// Look up the referenced type id.
	ttElem, err := d.typeDec.lookupType(tid)
	if err != nil {
		return nil, 0, err
	}
	var anyLen int
	if d.buf.version != Version80 {
		// Read and lookup the index of the any byte length.  Reference the any len,
		// even if it isn't used, to report missing references.
		lenIndex, err := binaryDecodeUint(d.buf)
		if err != nil {
			return nil, 0, err
		}
		if anyLen, err = d.refAnyLens.ReferencedAnyLen(lenIndex); err != nil {
			return nil, 0, err
		}
	}
	return ttElem, anyLen, nil
}

func (d *decoder81) skipValue(tt *vdl.Type) error { //nolint:gocyclo
	if tt.IsBytes() {
		len, err := binaryDecodeLenOrArrayLen(d.buf, tt)
		if err != nil {
			return err
		}
		return d.buf.Skip(len)
	}
	switch kind := tt.Kind(); kind {
	case vdl.Bool:
		return d.buf.Skip(1)
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Enum, vdl.TypeObject:
		// The underlying encoding of all these types is based on uint.
		return binarySkipUint(d.buf)
	case vdl.String:
		return binarySkipString(d.buf)
	case vdl.Array, vdl.List, vdl.Set, vdl.Map:
		len, err := binaryDecodeLenOrArrayLen(d.buf, tt)
		if err != nil {
			return err
		}
		for ix := 0; ix < len; ix++ {
			if kind == vdl.Set || kind == vdl.Map {
				if err := d.skipValue(tt.Key()); err != nil {
					return err
				}
			}
			if kind == vdl.Array || kind == vdl.List || kind == vdl.Map {
				if err := d.skipValue(tt.Elem()); err != nil {
					return err
				}
			}
		}
		return nil
	case vdl.Struct:
		// Loop through decoding the 0-based field index and corresponding field.
		for {
			switch ok, err := binaryDecodeControlOnly(d.buf, WireCtrlEnd); {
			case err != nil:
				return err
			case ok:
				return nil // end of struct
			}
			switch index, err := binaryDecodeUint(d.buf); {
			case err != nil:
				return err
			case index >= uint64(tt.NumField()):
				return errIndexOutOfRange
			default:
				ttfield := tt.Field(int(index))
				if err := d.skipValue(ttfield.Type); err != nil {
					return err
				}
			}
		}
	case vdl.Union:
		switch index, err := binaryDecodeUint(d.buf); {
		case err != nil:
			return err
		case index >= uint64(tt.NumField()):
			return errIndexOutOfRange
		default:
			ttfield := tt.Field(int(index))
			return d.skipValue(ttfield.Type)
		}
	case vdl.Optional:
		// Read the WireCtrlNil code, but if it's not WireCtrlNil we need to keep
		// the buffer as-is, since it's the first byte of the value, which may
		// itself be another control code.
		switch ctrl, err := binaryPeekControl(d.buf); {
		case err != nil:
			return err
		case ctrl == WireCtrlNil:
			d.buf.SkipAvailable(1) // nil optional
			return nil
		default:
			return d.skipValue(tt.Elem()) // non-nil optional
		}
	case vdl.Any:
		switch ok, err := binaryDecodeControlOnly(d.buf, WireCtrlNil); {
		case err != nil:
			return err
		case ok:
			return nil // nil any
		}
		switch index, err := binaryDecodeUint(d.buf); {
		case err != nil:
			return err
		default:
			tid, err := d.refTypes.ReferencedTypeID(index)
			if err != nil {
				return err
			}
			ttElem, err := d.typeDec.lookupType(tid)
			if err != nil {
				return err
			}
			return d.skipValue(ttElem)
		}
	default:
		return fmt.Errorf("vom: ignoreValue unhandled type %v", tt)
	}
}

func (d *decoder81) nextMessage() (TypeId, error) { //nolint:gocyclo
	if leftover := d.buf.RemoveLimit(); leftover > 0 {
		return 0, fmt.Errorf("vom: %v leftover bytes", leftover)
	}
	// Decode version byte, if not already decoded.
	if d.buf.version == 0 {
		version, err := d.buf.ReadByte()
		if err != nil {
			return 0, errEndedBeforeVersionByte(err)
		}
		d.buf.version = Version(version)
		if !isAllowedVersion(d.buf.version) {
			return 0, errBadVersionByte(d.buf.version)
		}
	}
	// Decode the next message id.
	incomplete, err := binaryDecodeControlOnly(d.buf, WireCtrlTypeIncomplete)
	if err != nil {
		return 0, err
	}
	mid, err := binaryDecodeInt(d.buf)
	if err != nil {
		return 0, err
	}
	if incomplete {
		if mid >= 0 {
			// TypeIncomplete must be followed with a type message.
			return 0, errInvalid
		}
		d.flag = d.flag.Set(decFlagTypeIncomplete)
	} else if mid < 0 {
		d.flag = d.flag.Clear(decFlagTypeIncomplete)
	}
	// TODO(toddw): Clean up the logic below.
	var tid TypeId
	var hasAny, hasTypeObject, hasLength bool
	switch {
	case mid < 0:
		tid = TypeId(-mid)
		hasLength = true
		hasAny = false
		hasTypeObject = false
	case mid > 0:
		tid = TypeId(mid)
		t, err := d.typeDec.lookupType(tid)
		if err != nil {
			return 0, err
		}
		hasLength = hasChunkLen(t)
		hasAny = containsAny(t)
		hasTypeObject = containsTypeObject(t)
	default:
		return 0, errDecodeZeroTypeID
	}

	if (hasAny || hasTypeObject) && d.buf.version != Version80 {
		l, err := binaryDecodeUint(d.buf)
		if err != nil {
			return 0, err
		}
		for i := 0; i < int(l); i++ {
			refID, err := binaryDecodeUint(d.buf)
			if err != nil {
				return 0, err
			}
			d.refTypes.AddTypeID(TypeId(refID))
		}
	}
	if hasAny && d.buf.version != Version80 {
		l, err := binaryDecodeUint(d.buf)
		if err != nil {
			return 0, err
		}
		for i := 0; i < int(l); i++ {
			refAnyLen, err := binaryDecodeLen(d.buf)
			if err != nil {
				return 0, err
			}
			d.refAnyLens.AddAnyLen(refAnyLen)
		}
	}

	if hasLength {
		chunkLen, err := binaryDecodeUint(d.buf)
		if err != nil {
			return 0, err
		}
		d.buf.SetLimit(int(chunkLen))
	}

	return tid, nil
}

func (d *decoder81) typeIsNext() (bool, error) {
	if d.buf.version == 0 {
		version, err := d.buf.ReadByte()
		if err != nil {
			return false, errEndedBeforeVersionByte(err)
		}
		d.buf.version = Version(version)
		if !isAllowedVersion(d.buf.version) {
			return false, errBadVersionByte(d.buf.version)
		}
	}
	switch ctrl, err := binaryPeekControl(d.buf); {
	case err != nil:
		return false, err
	case ctrl == WireCtrlTypeIncomplete:
		return true, nil
	case ctrl != 0:
		return false, errBadControlCode(ctrl)
	}
	mid, _, err := binaryPeekInt(d.buf)
	if err != nil {
		return false, err
	}
	return mid < 0, nil
}

func (d *decoder81) endMessage() error {
	if leftover := d.buf.RemoveLimit(); leftover > 0 {
		return fmt.Errorf("vom: %v leftover bytes", leftover)
	}
	if err := d.refTypes.Reset(); err != nil {
		return err
	}
	if err := d.refAnyLens.Reset(); err != nil {
		return err
	}
	return nil
}

type referencedTypes struct {
	tids   []TypeId
	marker int
}

func (refTypes *referencedTypes) Reset() (err error) {
	refTypes.tids = refTypes.tids[:0]
	refTypes.marker = 0
	return
}

func (refTypes *referencedTypes) AddTypeID(tid TypeId) {
	refTypes.tids = append(refTypes.tids, tid)
}

func (refTypes *referencedTypes) ReferencedTypeID(index uint64) (TypeId, error) {
	if index >= uint64(len(refTypes.tids)) {
		return 0, fmt.Errorf("vom: value referenced invalid index into type id table")
	}
	return refTypes.tids[index], nil
}

func (refTypes *referencedTypes) Mark() {
	refTypes.marker = len(refTypes.tids)
}

type referencedAnyLens struct {
	lens   []int
	marker int
}

func (refAnys *referencedAnyLens) Reset() (err error) {
	refAnys.lens = refAnys.lens[:0]
	return
}

func (refAnys *referencedAnyLens) AddAnyLen(len int) {
	refAnys.lens = append(refAnys.lens, len)
}

func (refAnys *referencedAnyLens) ReferencedAnyLen(index uint64) (int, error) {
	if index >= uint64(len(refAnys.lens)) {
		return 0, fmt.Errorf("vom: value referenced invalid index into anyLen table")
	}
	return refAnys.lens[index], nil
}

func (refAnys *referencedAnyLens) Mark() {
	refAnys.marker = len(refAnys.lens)
}
