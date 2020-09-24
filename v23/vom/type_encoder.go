// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"fmt"
	"io"
	"math"
	"sync"

	"v.io/v23/vdl"
)

// TypeEncoder manages the transmission and marshaling of types to the other
// side of a connection.
type TypeEncoder struct {
	typeMu   sync.RWMutex
	typeToID map[*vdl.Type]TypeId // GUARDED_BY(typeMu)
	nextID   TypeId               // GUARDED_BY(typeMu)

	encMu           sync.Mutex
	enc             *encoder81 // GUARDED_BY(encMu)
	sentVersionByte bool       // GUARDED_BY(encMu)
}

// NewTypeEncoder returns a new TypeEncoder that writes types to the given
// writer in the binary format.
func NewTypeEncoder(w io.Writer) *TypeEncoder {
	return NewVersionedTypeEncoder(DefaultVersion, w)
}

// NewTypeEncoderVersion returns a new TypeEncoder that writes types to the given
// writer in the specified VOM version.
func NewVersionedTypeEncoder(version Version, w io.Writer) *TypeEncoder {
	return &TypeEncoder{
		typeToID:        make(map[*vdl.Type]TypeId),
		nextID:          WireIdFirstUserType,
		enc:             newEncoderForTypes(version, w),
		sentVersionByte: false,
	}
}

func newTypeEncoderInternal(version Version, enc *encoder81) *TypeEncoder {
	return &TypeEncoder{
		typeToID:        make(map[*vdl.Type]TypeId),
		nextID:          WireIdFirstUserType,
		enc:             enc,
		sentVersionByte: true,
	}
}

// encode encodes the wire type tt recursively in depth-first order, encoding
// any children of the type before the type itself. Type ids are allocated in
// the order that we recurse and consequentially may be sent out of sequential
// order if type information for children is sent (before the parent type).
func (e *TypeEncoder) encode(tt *vdl.Type) (TypeId, error) {
	if tid := e.lookupTypeID(tt); tid != 0 {
		return tid, nil
	}

	// We serialize type encoding to avoid a race that can break our assumption
	// that all referenced types should be transmitted before the target type.
	// This can happen when we allow multiple flows encode types concurrently.
	// E.g.,
	//   * F1 is encoding T1
	//   * F2 is encoding T2 which has a T1 type field. F2 skipped T1 encoding,
	//     since F1 already assigned a type id to T1.
	//   * A type decoder can see T2 before T1 if F2 finishes encoding before F1.
	//
	// TODO(jhahn, toddw): We do not expect this would hurt the performance
	// practically. Revisit this if it becomes a real issue.
	//
	// TODO(jhahn, toddw): There is still a known race condition where multiple
	// flows send types with a cycle, but the types are referenced in different
	// orders. Figure out the solution.
	e.encMu.Lock()
	defer e.encMu.Unlock()
	if !e.sentVersionByte {
		if _, err := e.enc.writer.Write([]byte{byte(e.enc.version)}); err != nil {
			return 0, err
		}
		e.sentVersionByte = true
	}
	return e.encodeType(tt, map[*vdl.Type]bool{})
}

// encodeType encodes the type
func (e *TypeEncoder) encodeType(tt *vdl.Type, pending map[*vdl.Type]bool) (TypeId, error) { //nolint:gocyclo
	// Lookup a type Id for tt or assign a new one.
	tid, isNew, err := e.lookupOrAssignTypeID(tt)
	if err != nil {
		return 0, err
	}
	if !isNew {
		return tid, nil
	}
	pending[tt] = true

	// Construct the wireType.
	var wt wireType
	switch kind := tt.Kind(); kind {
	case vdl.Bool, vdl.Byte, vdl.String, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64:
		wt = wireTypeNamedT{wireNamed{tt.Name(), bootstrapKindToID[kind]}}
	case vdl.Enum:
		wireEnum := wireEnum{tt.Name(), make([]string, tt.NumEnumLabel())}
		for ix := 0; ix < tt.NumEnumLabel(); ix++ {
			wireEnum.Labels[ix] = tt.EnumLabel(ix)
		}
		wt = wireTypeEnumT{wireEnum}
	case vdl.Array:
		elm, err := e.encodeType(tt.Elem(), pending)
		if err != nil {
			return 0, err
		}
		wt = wireTypeArrayT{wireArray{tt.Name(), elm, uint64(tt.Len())}}
	case vdl.List:
		elm, err := e.encodeType(tt.Elem(), pending)
		if err != nil {
			return 0, err
		}
		wt = wireTypeListT{wireList{tt.Name(), elm}}
	case vdl.Set:
		key, err := e.encodeType(tt.Key(), pending)
		if err != nil {
			return 0, err
		}
		wt = wireTypeSetT{wireSet{tt.Name(), key}}
	case vdl.Map:
		key, err := e.encodeType(tt.Key(), pending)
		if err != nil {
			return 0, err
		}
		elm, err := e.encodeType(tt.Elem(), pending)
		if err != nil {
			return 0, err
		}
		wt = wireTypeMapT{wireMap{tt.Name(), key, elm}}
	case vdl.Struct:
		wireStruct := wireStruct{tt.Name(), make([]wireField, tt.NumField())}
		for ix := 0; ix < tt.NumField(); ix++ {
			field, err := e.encodeType(tt.Field(ix).Type, pending)
			if err != nil {
				return 0, err
			}
			wireStruct.Fields[ix] = wireField{tt.Field(ix).Name, field}
		}
		wt = wireTypeStructT{wireStruct}
	case vdl.Union:
		wireUnion := wireUnion{tt.Name(), make([]wireField, tt.NumField())}
		for ix := 0; ix < tt.NumField(); ix++ {
			field, err := e.encodeType(tt.Field(ix).Type, pending)
			if err != nil {
				return 0, err
			}
			wireUnion.Fields[ix] = wireField{tt.Field(ix).Name, field}
		}
		wt = wireTypeUnionT{wireUnion}
	case vdl.Optional:
		elm, err := e.encodeType(tt.Elem(), pending)
		if err != nil {
			return 0, err
		}
		wt = wireTypeOptionalT{wireOptional{tt.Name(), elm}}
	default:
		panic(fmt.Errorf("vom: encode unhandled type %v", tt))
	}

	// TODO(bprosnitz) Only perform the walk when there are cycles or otherwise optimize this
	delete(pending, tt)
	typeComplete := tt.Walk(vdl.WalkAll, func(t *vdl.Type) bool {
		return !pending[t]
	})

	// Encode and write the wire type definition using the same
	// binary encoder as values for wire types.
	if err := e.enc.encodeWireType(tid, wt, !typeComplete); err != nil {
		return 0, err
	}

	return tid, nil
}

// lookupTypeID returns the id for the type tt if it is already encoded;
// otherwise zero id is returned.
func (e *TypeEncoder) lookupTypeID(tt *vdl.Type) TypeId {
	if tid := bootstrapTypeToID[tt]; tid != 0 {
		return tid
	}
	e.typeMu.RLock()
	tid := e.typeToID[tt]
	e.typeMu.RUnlock()
	return tid
}

func (e *TypeEncoder) lookupOrAssignTypeID(tt *vdl.Type) (TypeId, bool, error) {
	if tid := bootstrapTypeToID[tt]; tid != 0 {
		return tid, false, nil
	}
	e.typeMu.Lock()
	tid := e.typeToID[tt]
	if tid > 0 {
		e.typeMu.Unlock()
		return tid, false, nil
	}

	// Assign a new id.
	newID := e.nextID
	if newID > math.MaxInt64 {
		e.typeMu.Unlock()
		return 0, false, fmt.Errorf("vom: encoder type id overflow")
	}
	e.nextID++
	e.typeToID[tt] = newID
	e.typeMu.Unlock()
	return newID, true, nil
}

func (e *TypeEncoder) makeIDToTypeUnlocked() map[TypeId]*vdl.Type {
	if len(e.typeToID) == 0 {
		return nil
	}
	result := make(map[TypeId]*vdl.Type, len(e.typeToID))
	for tt, id := range e.typeToID {
		result[id] = tt
	}
	return result
}
