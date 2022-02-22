// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

func convertValue(dst, src interface{}) error {
	var v Value
	enc := newToValueEncoder(&v)
	if err := Write(enc, src); err != nil {
		return err
	}
	return Read(v.Decoder(), dst)
}

func convertValueReflect(dst, src reflect.Value) error {
	var v Value
	enc := newToValueEncoder(&v)
	if err := WriteReflect(enc, src); err != nil {
		return err
	}
	return ReadReflect(v.Decoder(), dst)
}

func newToValueEncoder(value *Value) *toValueEncoder {
	e := &toValueEncoder{value: value}
	e.stack = e.stackStorage[:0]
	return e
}

var (
	errEmptyEncoderStack = errors.New("vdl: empty 'to value' encoder stack")
	errFinishCalledEarly = errors.New("vdl: 'to value' encoder finish called before a composite type is complete")
)

type toValueEncoder struct {
	stackStorage             [4]tveStackEntry
	stack                    []tveStackEntry
	value                    *Value
	nextStartValueIsOptional bool
}

type tveAssignmentType int

const (
	primitiveValue tveAssignmentType = 0
	optionalValue  tveAssignmentType = 1 << iota
	fieldsValue
	indexedValue
	setValue
	mapValue
	enumValue
)

func (at tveAssignmentType) String() string {
	var s string
	switch at & ^optionalValue {
	case primitiveValue:
		s = "primitive"
	case fieldsValue:
		s = "fields"
	case indexedValue:
		s = "indexed"
	case setValue:
		s = "set"
	case mapValue:
		s = "map"
	case enumValue:
		s = "enum"
	}
	if (at & optionalValue) != 0 {
		return "?" + s
	}
	return s
}

func (entry *tveStackEntry) String() string {
	out := &strings.Builder{}
	fmt.Fprintf(out, "%v: %v: assignment: %v, field: %v, entry %v, mapIndex %v, lenHint: %v", entry.Value.t, entry.Value.rep, entry.typeAssignment, entry.nextFieldIndex, entry.nextIndex, entry.mapKey.t != nil, entry.lenHint)
	return out.String()
}

func formatStack(stack []tveStackEntry) string { //nolint:unused
	out := &strings.Builder{}
	for i, s := range stack {
		fmt.Fprintf(out, "%v: %s\n", i, s.String())
	}
	return out.String()
}

func calledFrom(msg string, skip int) { //nolint:unused
	pcs := make([]uintptr, 10)
	n := runtime.Callers(skip+1, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	frame, _ := frames.Next()
	fmt.Printf("%v: called from: %v\n", msg, frame.Function)
}

func (e *toValueEncoder) callingFunction(msg string, tt *Type, showStack bool) { //nolint:unused
	calledFrom(msg, 3)
	if tt != nil {
		fmt.Printf("%v: %v: %v\n", msg, tt.String(), tt.kind)
	}
	if showStack {
		fmt.Printf("%v: encoder stack [\n", msg)
		fmt.Printf("%s]: %s\n\n", formatStack(e.stack), msg)
	}
}

type tveStackEntry struct {
	Value

	typeAssignment tveAssignmentType
	nextFieldIndex int   // needed for structs.
	nextIndex      int   // needed fors lists, sets etc.
	mapKey         Value // needed for maps.
	lenHint        int
}

func (entry *tveStackEntry) encodePrimitive(valueRep interface{}) error {
	if entry == nil {
		return errEmptyEncoderStack
	}
	entry.Value.rep = valueRep
	return nil
}

func (entry *tveStackEntry) asssignStringOrEnum(value string) {
	if entry.typeAssignment == enumValue {
		entry.Value.rep = enumIndex(entry.Value.t.EnumIndex(value))
	} else {
		entry.Value.rep = value
	}
}

// A string can also be an enum.
func (entry *tveStackEntry) encodeStringPrimitive(value string) error {
	if entry == nil {
		return errEmptyEncoderStack
	}
	entry.asssignStringOrEnum(value)
	return nil
}

func (e *toValueEncoder) top() *tveStackEntry {
	if len(e.stack) == 0 {
		return nil
	}
	return &e.stack[len(e.stack)-1]
}

func determineAssignmentAndRep(tt *Type) (assignmentType tveAssignmentType, rep interface{}) {
	if tt.IsBytes() {
		// Special case bytes as a primitive value.
		// The bytes representation is filled in later.
		assignmentType = primitiveValue
		return
	}
	switch tt.kind {
	case Struct:
		assignmentType = fieldsValue
		rep = zeroRepSequence(len(tt.fields))
	case Union:
		assignmentType = fieldsValue
		rep = &repUnion{0, ZeroValue(tt.fields[0].Type)}
	case Array:
		assignmentType = indexedValue
		rep = zeroRepSequence(tt.len)
	case List:
		assignmentType = indexedValue
		rep = zeroRepSequence(0)
	case Set:
		assignmentType = setValue
		rep = zeroRepMap(tt.key)
	case Map:
		assignmentType = mapValue
		rep = zeroRepMap(tt.key)
	case Enum:
		assignmentType = enumValue
		rep = enumIndex(0)
	default:
		rep = zeroRepKindMap[tt.kind]
	}
	return
}

func (e *toValueEncoder) start(tt *Type) *tveStackEntry {
	assignmentType, rep := determineAssignmentAndRep(tt)
	if e.nextStartValueIsOptional {
		assignmentType |= optionalValue
	}

	e.stack = append(e.stack, tveStackEntry{
		Value:          Value{t: tt, rep: rep},
		typeAssignment: assignmentType,
		nextFieldIndex: -1,
		nextIndex:      -1,
		lenHint:        -1,
	})
	e.nextStartValueIsOptional = false
	return &e.stack[len(e.stack)-1]
}

func (entry *tveStackEntry) optionalAssignee(t *Type) {
	if t.kind == Optional && entry.t.kind != Optional {
		entry.Value = *OptionalValue(&entry.Value)
	}
}

// The stack must not be empty when this is called and
// a composite type must be complete. Checking for both
// conditions is external to this function.
func (e *toValueEncoder) finish() error {
	top := e.stack[len(e.stack)-1]

	if (top.typeAssignment & optionalValue) != 0 {
		top.Value = *OptionalValue(&top.Value)
	}

	if len(e.stack) == 1 {
		// save the last value so that it can be retrieved.
		*e.value = top.Value
		e.stack = e.stack[:0]
		return nil
	}
	parent := &e.stack[len(e.stack)-2]

	if parent.typeAssignment == primitiveValue {
		parent.Value.rep = top.Value.rep
		e.stack = e.stack[:len(e.stack)-1]
		return nil
	}

	switch parent.typeAssignment & ^optionalValue {
	case fieldsValue:
		field := parent.t.Field(parent.nextFieldIndex)
		top.optionalAssignee(field.Type)
		parent.AssignField(parent.nextFieldIndex, &top.Value)
	case indexedValue:
		top.optionalAssignee(parent.t.elem)
		parent.AssignIndex(parent.nextIndex, &top.Value)
	case setValue:
		top.optionalAssignee(parent.t)
		parent.AssignSetKey(&top.Value)
	case mapValue:
		if parent.mapKey.t != nil {
			parent.AssignMapIndex(&parent.mapKey, &top.Value)
			parent.mapKey = Value{}
		} else {
			parent.mapKey = top.Value
		}
	}
	e.stack = e.stack[:len(e.stack)-1]
	return nil
}

func (e *toValueEncoder) StartValue(tt *Type) error {
	e.start(tt)
	return nil
}

func (e *toValueEncoder) FinishValue() error {
	if len(e.stack) == 0 {
		return errEmptyEncoderStack
	}
	top := e.top()
	if top.nextIndex != -1 || top.nextFieldIndex != -1 {
		return errFinishCalledEarly
	}
	return e.finish()
}

func (e *toValueEncoder) NilValue(tt *Type) error {
	switch tt.Kind() {
	case Any, Optional:
	default:
		return fmt.Errorf("concrete types disallowed for NilValue (type was %v)", tt)
	}
	assignmentType, _ := determineAssignmentAndRep(tt)
	e.stack = append(e.stack, tveStackEntry{
		Value:          Value{t: tt, rep: (*Value)(nil)},
		typeAssignment: assignmentType,
		nextFieldIndex: -1,
		nextIndex:      -1,
		lenHint:        -1,
	})
	e.nextStartValueIsOptional = false
	return e.finish()
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
		return nil
	}
	top.nextEntry()
	return nil
}

func (e *toValueEncoder) NextField(index int) error {
	if top := e.top(); top != nil {
		top.nextFieldIndex = index
		return nil
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) SetLenHint(lenHint int) error {
	if top := e.top(); top != nil {
		top.lenHint = lenHint
		return nil
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) EncodeBool(value bool) error {
	return e.top().encodePrimitive(value)
}

func (e *toValueEncoder) EncodeString(value string) error {
	return e.top().encodeStringPrimitive(value)
}

func (e *toValueEncoder) EncodeUint(value uint64) error {
	return e.top().encodePrimitive(value)
}

func (e *toValueEncoder) EncodeInt(value int64) error {
	return e.top().encodePrimitive(value)
}

func (e *toValueEncoder) EncodeFloat(value float64) error {
	return e.top().encodePrimitive(value)
}

func (e *toValueEncoder) EncodeTypeObject(value *Type) error {
	return e.top().encodePrimitive(value)
}

func (e *toValueEncoder) EncodeBytes(value []byte) error {
	return e.top().encodeBytes(value)
}

func (entry *tveStackEntry) encodeBytes(value []byte) error {
	if entry == nil {
		return errEmptyEncoderStack
	}
	entry.Value.rep = createBytesRep(entry.lenHint, entry.Value.t.len, value)
	return nil
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

func (e *toValueEncoder) WriteValueBool(tt *Type, value bool) error {
	top := e.start(tt)
	top.Value.rep = value
	return e.finish()
}

func (e *toValueEncoder) WriteValueString(tt *Type, value string) error {
	top := e.start(tt)
	top.asssignStringOrEnum(value)
	return e.finish()
}

func (e *toValueEncoder) WriteValueUint(tt *Type, value uint64) error {
	top := e.start(tt)
	top.Value.rep = value
	return e.finish()
}

func (e *toValueEncoder) WriteValueInt(tt *Type, value int64) error {
	top := e.start(tt)
	top.Value.rep = value
	return e.finish()
}

func (e *toValueEncoder) WriteValueFloat(tt *Type, value float64) error {
	top := e.start(tt)
	top.Value.rep = value
	return e.finish()
}

func (e *toValueEncoder) WriteValueTypeObject(value *Type) error {
	top := e.start(TypeObjectType)
	top.Value.rep = value
	return e.finish()
}

func (e *toValueEncoder) WriteValueBytes(tt *Type, value []byte) error {
	top := e.start(tt)
	top.Value.rep = createBytesRep(top.lenHint, tt.len, value)
	return e.finish()
}

func (entry *tveStackEntry) nextEntry() {
	entry.nextIndex++
	if entry.lenHint != -1 {
		if entry.typeAssignment == indexedValue && entry.t.kind != Array {
			entry.AssignLen(entry.lenHint)
		}
		entry.lenHint = -1
	}
}

func (e *toValueEncoder) NextEntryValueBool(tt *Type, value bool) error {
	if top := e.top(); top != nil {
		top.nextEntry()
		e.start(tt).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextEntryValueString(tt *Type, value string) error {
	if top := e.top(); top != nil {
		top.nextEntry()
		e.start(tt).asssignStringOrEnum(value)
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextEntryValueUint(tt *Type, value uint64) error {
	if top := e.top(); top != nil {
		top.nextEntry()
		e.start(tt).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextEntryValueInt(tt *Type, value int64) error {
	if top := e.top(); top != nil {
		top.nextEntry()
		e.start(tt).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextEntryValueFloat(tt *Type, value float64) error {
	if top := e.top(); top != nil {
		top.nextEntry()
		e.start(tt).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextEntryValueTypeObject(value *Type) error {
	if top := e.top(); top != nil {
		top.nextEntry()
		e.start(TypeObjectType).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextEntryValueBytes(tt *Type, value []byte) error {
	if top := e.top(); top != nil {
		top.nextEntry()
		e.start(tt).Value.rep = createBytesRep(top.lenHint, tt.len, value)
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextFieldValueBool(index int, tt *Type, value bool) error {
	if top := e.top(); top != nil {
		top.nextFieldIndex = index
		e.start(tt).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextFieldValueString(index int, tt *Type, value string) error {
	if top := e.top(); top != nil {
		top.nextFieldIndex = index
		e.start(tt).asssignStringOrEnum(value)
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextFieldValueUint(index int, tt *Type, value uint64) error {
	if top := e.top(); top != nil {
		top.nextFieldIndex = index
		e.start(tt).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextFieldValueInt(index int, tt *Type, value int64) error {
	if top := e.top(); top != nil {
		top.nextFieldIndex = index
		e.start(tt).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextFieldValueFloat(index int, tt *Type, value float64) error {
	if top := e.top(); top != nil {
		top.nextFieldIndex = index
		e.start(tt).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextFieldValueTypeObject(index int, value *Type) error {
	if top := e.top(); top != nil {
		top.nextFieldIndex = index
		e.start(TypeObjectType).Value.rep = value
		return e.finish()
	}
	return errEmptyEncoderStack
}

func (e *toValueEncoder) NextFieldValueBytes(index int, tt *Type, value []byte) error {
	if top := e.top(); top != nil {
		top.nextFieldIndex = index
		e.start(tt).Value.rep = createBytesRep(top.lenHint, tt.len, value)
		return e.finish()
	}
	return errEmptyEncoderStack
}
