package vdl

import (
	"errors"
	"fmt"
	"reflect"
)

func convertValue(dst, src interface{}) error {
	enc := newToValueEncoder()
	if err := Write(enc, src); err != nil {
		return err
	}
	v := enc.Value()
	return Read(v.Decoder(), dst)
}

func convertValueReflect(dst, src reflect.Value) error {
	enc := newToValueEncoder()
	if err := WriteReflect(enc, src); err != nil {
		return err
	}
	v := enc.Value()
	return ReadReflect(v.Decoder(), dst)
}

func newToValueEncoder() *toValueEncoder {
	e := &toValueEncoder{}
	e.stack = e.stackStorage[:0]
	return e
}

var errEmptyEncoderStack = errors.New("vdl: empty to value encoder stack")

type tveTypeMask int

const (
	primitiveValue tveTypeMask = 0
	fieldsValue    tveTypeMask = 1 << iota
	indexedValue
	setValue
	mapValue
	optionalValue
)

type tveStackEntry struct {
	Value

	typeMask       tveTypeMask
	mapValue       Value
	nextType       *Type
	nextFieldIndex int
	nextIndex      int
	lenHint        int
}

func (entry *tveStackEntry) encode(valueRep interface{}) error {
	if entry == nil {
		return errEmptyEncoderStack
	}
	if entry.typeMask == primitiveValue {
		entry.Value.rep = valueRep
		return nil
	}
	value := Value{t: entry.nextType, rep: valueRep}
	entry.assignValue(&value)
	return nil
}

func (entry *tveStackEntry) encodeBytes(value []byte) error {
	if entry == nil {
		return errEmptyEncoderStack
	}
	rep := createBytesRep(entry.lenHint, entry.Value.t.len, value)
	if entry.typeMask == primitiveValue {
		entry.Value.rep = rep
		return nil
	}
	vv := Value{t: entry.nextType, rep: rep}
	entry.assignValue(&vv)
	return nil
}

//go:noinline
func (entry *tveStackEntry) assignValue(v *Value) {
	switch {
	case (entry.typeMask & fieldsValue) != 0:
		entry.AssignField(entry.nextFieldIndex, v)
	case (entry.typeMask & indexedValue) != 0:
		entry.AssignIndex(entry.nextIndex, v)
	case (entry.typeMask & setValue) != 0:
		entry.AssignSetKey(v)
	case (entry.typeMask & mapValue) != 0:
		if entry.nextIndex == 1 {
			entry.AssignMapIndex(v, &entry.mapValue)
		} else {
			entry.mapValue = *v
		}
	default:
		panic("not implemented")
	}
}

type toValueEncoder struct {
	stackStorage             [4]tveStackEntry
	stack                    []tveStackEntry
	value                    Value
	nextStartValueIsOptional bool
}

func (e *toValueEncoder) reset() {
	e.stack = e.stackStorage[:0]
	e.value = Value{}
	e.nextStartValueIsOptional = false
}

func (e *toValueEncoder) top() *tveStackEntry {
	if len(e.stack) == 0 {
		return nil
	}
	return &e.stack[len(e.stack)-1]
}

func (e *toValueEncoder) start(tt *Type) *tveStackEntry {
	if len(e.stack) > 0 {
		top := &e.stack[len(e.stack)-1]
		if top.typeMask != primitiveValue && tt {
			top.nextType = tt
			return top
		}
	}
	ttkind := tt.Kind()
	var mask tveTypeMask
	var rep interface{}
	fmt.Printf("TT... %v\n", tt)
	switch {
	case tt.IsBytes():
		// Special case bytes as a primitive value.
	case ttkind == Struct || ttkind == Union:
		mask |= fieldsValue
		rep = zeroRep(tt)
	case ttkind == Array || ttkind == List:
		mask |= indexedValue
		rep = zeroRep(tt)
	case ttkind == Set:
		mask |= setValue
		rep = zeroRep(tt)
	case ttkind == Map:
		mask |= mapValue
		rep = zeroRep(tt)
	case e.nextStartValueIsOptional:
		mask |= optionalValue
		rep = zeroRep(tt)
	}
	e.stack = append(e.stack, tveStackEntry{
		Value:          Value{t: tt, rep: rep},
		typeMask:       mask,
		nextFieldIndex: -1,
		nextIndex:      -1,
		lenHint:        -1,
	})
	e.nextStartValueIsOptional = false
	return &e.stack[len(e.stack)-1]
}

func (e *toValueEncoder) finish() {
	if len(e.stack) == 0 {
		return
	}
	top := e.stack[len(e.stack)-1]
	if len(e.stack) == 1 {
		// save the last value so that it can be retrieved.
		e.value = top.Value
	}
	// what about maps?
	if top.typeMask == primitiveValue || (top.nextFieldIndex == -1 && top.nextIndex == -1) {
		e.stack = e.stack[:len(e.stack)-1]
	}
}

func (e *toValueEncoder) Value() Value {
	return e.value
}

func (e *toValueEncoder) StartValue(tt *Type) error {
	if e.nextStartValueIsOptional {
		panic("x")
	}
	e.start(tt)
	return nil
}

func (e *toValueEncoder) FinishValue() error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	e.finish()
	return nil
}

func (e *toValueEncoder) NilValue(tt *Type) error {
	switch tt.Kind() {
	case Any:
	case Optional:
		e.SetNextStartValueIsOptional()
	default:
		return fmt.Errorf("concrete types disallowed for NilValue (type was %v)", tt)
	}
	top := e.start(tt)
	top.Value.rep = nil
	e.finish()
	return nil
}

func (e *toValueEncoder) SetNextStartValueIsOptional() {
	e.nextStartValueIsOptional = true
}

func (e *toValueEncoder) NextEntry(done bool) error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	if done {
		top.nextIndex = -1
		top.mapValue = Value{}
		return nil
	}
	if top.lenHint != -1 {
		top.AssignLen(top.lenHint)
		top.lenHint = -1
	}
	top.nextIndex++
	return nil
}

func (e *toValueEncoder) NextField(index int) error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	top.nextFieldIndex = index
	fmt.Printf("TOP: %#v\n", top)
	return nil
}

func (e *toValueEncoder) SetLenHint(lenHint int) error {
	if top := e.top(); top != nil {
		top.lenHint = lenHint
		return nil
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) EncodeBool(value bool) error {
	return e.top().encode(value)
}

func (e *toValueEncoder) EncodeString(value string) error {
	return e.top().encode(value)
}

func (e *toValueEncoder) EncodeUint(value uint64) error {
	return e.top().encode(value)
}

func (e *toValueEncoder) EncodeInt(value int64) error {
	return e.top().encode(value)
}

func (e *toValueEncoder) EncodeFloat(value float64) error {
	return e.top().encode(value)
}

func (e *toValueEncoder) EncodeTypeObject(value *Type) error {
	return e.top().encode(value)
}

func createBytesRep(lenHint, arraySize int, value []byte) *repBytes {
	var bytesVal repBytes
	switch {
	case lenHint != -1: // list
		bytesVal = make(repBytes, 0, lenHint)
		bytesVal = append(bytesVal, value...)
	case arraySize != 0: // array
		bytesVal = make(repBytes, arraySize)
		copy(bytesVal, value)
	case lenHint == -1: // list with no size hint.
		bytesVal = make(repBytes, len(value))
		copy(bytesVal, value)
	}
	return &bytesVal
}

func (e *toValueEncoder) EncodeBytes(value []byte) error {
	return e.top().encodeBytes(value)
}

func (e *toValueEncoder) WriteValueBool(tt *Type, value bool) error {
	top := e.start(tt)
	if err := top.encode(value); err != nil {
		return err
	}
	e.finish()
	return nil
}

func (e *toValueEncoder) WriteValueString(tt *Type, value string) error {
	top := e.start(tt)
	if err := top.encode(value); err != nil {
		return err
	}
	e.finish()
	return nil
}

func (e *toValueEncoder) WriteValueUint(tt *Type, value uint64) error {
	top := e.start(tt)
	if err := top.encode(value); err != nil {
		return err
	}
	e.finish()
	return nil
}

func (e *toValueEncoder) WriteValueInt(tt *Type, value int64) error {
	top := e.start(tt)
	if err := top.encode(value); err != nil {
		return err
	}
	e.finish()
	return nil
}

func (e *toValueEncoder) WriteValueFloat(tt *Type, value float64) error {
	top := e.start(tt)
	if err := top.encode(value); err != nil {
		return err
	}
	e.finish()
	return nil
}

func (e *toValueEncoder) WriteValueTypeObject(value *Type) error {
	top := e.start(TypeObjectType)
	if err := top.encode(value); err != nil {
		return err
	}
	e.finish()
	return nil
}

func (e *toValueEncoder) WriteValueBytes(tt *Type, value []byte) error {
	top := e.start(tt)
	if err := top.encode(createBytesRep(top.lenHint, tt.len, value)); err != nil {
		return err
	}
	e.finish()
	return nil
}

func (e *toValueEncoder) NextEntryValueBool(tt *Type, value bool) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextEntryValueString(tt *Type, value string) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextEntryValueUint(tt *Type, value uint64) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextEntryValueInt(tt *Type, value int64) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextEntryValueFloat(tt *Type, value float64) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextEntryValueTypeObject(value *Type) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextEntryValueBytes(tt *Type, value []byte) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextFieldValueBool(index int, tt *Type, value bool) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextFieldValueString(index int, tt *Type, value string) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextFieldValueUint(index int, tt *Type, value uint64) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextFieldValueInt(index int, tt *Type, value int64) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextFieldValueFloat(index int, tt *Type, value float64) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextFieldValueTypeObject(index int, value *Type) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}

func (e *toValueEncoder) NextFieldValueBytes(index int, tt *Type, value []byte) error {
	fmt.Printf("X.. %v\n", value)
	return nil
}
