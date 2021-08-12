// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package verror

import (
	"v.io/v23/vdl"
)

func init() {
	vdl.RegisterNative(WireToNative, WireFromNative)
	vdl.RegisterNativeAnyType(vdl.WireError{}, (*E)(nil))
	vdl.RegisterNativeConverter(&vdl.WireError{}, VDLNativeConverter{})
}

type VDLNativeConverter struct{}

// ToNative implements vdl.NativeConverter.
func (VDLNativeConverter) ToNative(wire, native interface{}) error {
	return WireToNative(wire.(*vdl.WireError), native.(*error))
}

// FromNative implements vdl.NativeConverter.
func (VDLNativeConverter) FromNative(wire, native interface{}) error {
	return WireFromNative(wire.(**vdl.WireError), native.(error))
}

// WireToNative converts from the wire to native representation of errors.
func WireToNative(wire *vdl.WireError, native *error) error {
	if wire == nil {
		*native = nil
		return nil
	}
	e := &E{
		ID:     ID(wire.Id),
		Action: retryToAction(wire.RetryCode),
		Msg:    wire.Msg,
	}
	e.ParamList = make([]interface{}, 0, len(wire.ParamList))
	if err := vdl.Convert(&e.ParamList, wire.ParamList); err != nil {
		// It's questionable what to do here if the conversion fails, similarly to
		// the conversion failure below in WireFromNative.
		//
		// TODO(toddw): Consider whether there is a better strategy.
		for _, w := range wire.ParamList {
			e.ParamList = append(e.ParamList, w)
		}
	}
	*native = e
	return nil
}

// WireFromNative converts from the native to wire representation of errors.
func WireFromNative(wire **vdl.WireError, native error) error {
	var e E
	switch v := native.(type) {
	case E:
		e = v
	case *E:
		e = *v
	default:
		nerr := native.Error()
		e = E{
			ID:        ErrUnknown.ID,
			Action:    NoRetry,
			Msg:       nerr,
			ParamList: []interface{}{"", "", nerr}}
	}
	nt := *wire
	if nt == nil {
		nt = new(vdl.WireError)
		*wire = nt
	}
	nt.Id = string(e.ID)
	nt.RetryCode = retryFromAction(e.Action)
	nt.Msg = e.Msg
	nt.ParamList = make([]*vdl.Value, 0, len(e.ParamList))
	if err := vdl.Convert(&nt.ParamList, e.ParamList); err != nil {
		// It's questionable what to do here if the conversion fails, similarly to
		// the conversion failure above in WireToNative.
		//
		// TODO(toddw): Consider whether there is a better strategy.
		nt.ParamList = append(nt.ParamList,
			vdl.StringValue(nil, ""),
			vdl.StringValue(nil, ""),
			vdl.StringValue(nil, err.Error()),
		)
	}
	return nil
}

// FromWire is a convenience for generated code to convert wire errors into
// native errors.
func FromWire(wire *vdl.WireError) error {
	if wire == nil {
		return nil
	}
	var native error
	if err := WireToNative(wire, &native); err != nil {
		native = err
	}
	return native
}

var (
	retryToActionMap = map[vdl.WireRetryCode]ActionCode{
		vdl.WireRetryCodeNoRetry:         NoRetry,
		vdl.WireRetryCodeRetryConnection: RetryConnection,
		vdl.WireRetryCodeRetryRefetch:    RetryRefetch,
		vdl.WireRetryCodeRetryBackoff:    RetryBackoff,
	}
	retryFromActionMap = map[ActionCode]vdl.WireRetryCode{
		NoRetry:         vdl.WireRetryCodeNoRetry,
		RetryConnection: vdl.WireRetryCodeRetryConnection,
		RetryRefetch:    vdl.WireRetryCodeRetryRefetch,
		RetryBackoff:    vdl.WireRetryCodeRetryBackoff,
	}
)

func retryToAction(retry vdl.WireRetryCode) ActionCode {
	if r, ok := retryToActionMap[retry]; ok {
		return r
	}
	// Backoff to no retry by default.
	return NoRetry
}

func retryFromAction(action ActionCode) vdl.WireRetryCode {
	if r, ok := retryFromActionMap[action]; ok {
		return r
	}
	// Backoff to no retry by default.
	return vdl.WireRetryCodeNoRetry
}

// VDLRead implements the logic to read x from dec.
//
// Unlike regular VDLRead implementations, this handles the case where the
// decoder contains a nil value, to make code generation simpler.
func VDLRead(dec vdl.Decoder, x *error) error {
	if err := dec.StartValue(vdl.ErrorType.Elem()); err != nil {
		return err
	}
	if dec.IsNil() {
		*x = nil
		return dec.FinishValue()
	}
	dec.IgnoreNextStartValue()
	var wire vdl.WireError
	if err := wire.VDLRead(dec); err != nil {
		return err
	}
	return WireToNative(&wire, x)
}

// VDLWrite implements the logic to write x to enc.
//
// Unlike regular VDLWrite implementations, this handles the case where x
// contains a nil value, to make code generation simpler.
func VDLWrite(enc vdl.Encoder, x error) error {
	if x == nil {
		return enc.NilValue(vdl.ErrorType)
	}
	var wire vdl.WireError
	wirePtr := &wire
	if err := WireFromNative(&wirePtr, x); err != nil {
		return err
	}
	return wire.VDLWrite(enc)
}
