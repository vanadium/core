// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: bcrypter

//nolint:revive
package bcrypter

import (
	"fmt"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

// Type definitions
// ================

// WireCiphertext represents the wire format of the ciphertext
// generated by a Crypter.
type WireCiphertext struct {
	// PatternId is an identifier of the blessing pattern that this
	// ciphertext is for. It is represented by a 16 byte truncated
	// SHA256 hash of the pattern.
	PatternId string
	// Bytes is a map from an identifier of the public IBE params to
	// the ciphertext bytes that were generated using those params.
	//
	// The params identifier is a 16 byte truncated SHA256 hash
	// of the marshaled form of the IBE params.
	Bytes map[string][]byte
}

func (WireCiphertext) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/security/bcrypter.WireCiphertext"`
}) {
}

func (x WireCiphertext) VDLIsZero() bool { //nolint:gocyclo
	if x.PatternId != "" {
		return false
	}
	if len(x.Bytes) != 0 {
		return false
	}
	return true
}

func (x WireCiphertext) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct1); err != nil {
		return err
	}
	if x.PatternId != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.PatternId); err != nil {
			return err
		}
	}
	if len(x.Bytes) != 0 {
		if err := enc.NextField(1); err != nil {
			return err
		}
		if err := vdlWriteAnonMap1(enc, x.Bytes); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func vdlWriteAnonMap1(enc vdl.Encoder, x map[string][]byte) error {
	if err := enc.StartValue(vdlTypeMap2); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for key, elem := range x {
		if err := enc.NextEntryValueString(vdl.StringType, key); err != nil {
			return err
		}
		if err := enc.WriteValueBytes(vdlTypeList3, elem); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WireCiphertext) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = WireCiphertext{}
	if err := dec.StartValue(vdlTypeStruct1); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != vdlTypeStruct1 {
			index = vdlTypeStruct1.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.PatternId = value
			}
		case 1:
			if err := vdlReadAnonMap1(dec, &x.Bytes); err != nil {
				return err
			}
		}
	}
}

func vdlReadAnonMap1(dec vdl.Decoder, x *map[string][]byte) error {
	if err := dec.StartValue(vdlTypeMap2); err != nil {
		return err
	}
	var tmpMap map[string][]byte
	if len := dec.LenHint(); len > 0 {
		tmpMap = make(map[string][]byte, len)
	}
	for {
		switch done, key, err := dec.NextEntryValueString(); {
		case err != nil:
			return err
		case done:
			*x = tmpMap
			return dec.FinishValue()
		default:
			var elem []byte
			if err := dec.ReadValueBytes(-1, &elem); err != nil {
				return err
			}
			if tmpMap == nil {
				tmpMap = make(map[string][]byte)
			}
			tmpMap[key] = elem
		}
	}
}

// WireParams represents the wire format of the public parameters
// of an identity provider (aka Root).
type WireParams struct {
	// Blessing is the blessing name of the identity provider. The identity
	// provider  can extract private keys for blessings that are extensions
	// of this blessing name.
	Blessing string
	// Params is the marshaled form of the public IBE params of the
	// the identity provider.
	Params []byte
}

func (WireParams) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/security/bcrypter.WireParams"`
}) {
}

func (x WireParams) VDLIsZero() bool { //nolint:gocyclo
	if x.Blessing != "" {
		return false
	}
	if len(x.Params) != 0 {
		return false
	}
	return true
}

func (x WireParams) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct4); err != nil {
		return err
	}
	if x.Blessing != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Blessing); err != nil {
			return err
		}
	}
	if len(x.Params) != 0 {
		if err := enc.NextFieldValueBytes(1, vdlTypeList3, x.Params); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WireParams) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = WireParams{}
	if err := dec.StartValue(vdlTypeStruct4); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != vdlTypeStruct4 {
			index = vdlTypeStruct4.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Blessing = value
			}
		case 1:
			if err := dec.ReadValueBytes(-1, &x.Params); err != nil {
				return err
			}
		}
	}
}

// WirePrivateKey represents the wire format of the private key corresponding
// to a blessing.
type WirePrivateKey struct {
	// Blessing is the blessing for which this private key was extracted for.
	Blessing string
	// Params are the public parameters of the identity provider that extracted
	// this private key.
	Params WireParams
	// Keys contain the extracted IBE private keys for each pattern that is
	// matched by the blessing and is an extension of the identity provider's
	// name. The keys are enumerated in increasing order of the lengths of the
	// corresponding patterns.
	//
	// For example, if the blessing is "google:u:alice:phone" and the identity
	// provider's name is "google:u" then the keys are extracted for the patterns
	// - "google:u"
	// - "google:u:alice"
	// - "google:u:alice:phone"
	// - "google:u:alice:phone:$"
	//
	// The private keys are listed in increasing order of the lengths of the
	// corresponding patterns.
	Keys [][]byte
}

func (WirePrivateKey) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/security/bcrypter.WirePrivateKey"`
}) {
}

func (x WirePrivateKey) VDLIsZero() bool { //nolint:gocyclo
	if x.Blessing != "" {
		return false
	}
	if !x.Params.VDLIsZero() {
		return false
	}
	if len(x.Keys) != 0 {
		return false
	}
	return true
}

func (x WirePrivateKey) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct5); err != nil {
		return err
	}
	if x.Blessing != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Blessing); err != nil {
			return err
		}
	}
	if !x.Params.VDLIsZero() {
		if err := enc.NextField(1); err != nil {
			return err
		}
		if err := x.Params.VDLWrite(enc); err != nil {
			return err
		}
	}
	if len(x.Keys) != 0 {
		if err := enc.NextField(2); err != nil {
			return err
		}
		if err := vdlWriteAnonList2(enc, x.Keys); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func vdlWriteAnonList2(enc vdl.Encoder, x [][]byte) error {
	if err := enc.StartValue(vdlTypeList6); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for _, elem := range x {
		if err := enc.NextEntryValueBytes(vdlTypeList3, elem); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WirePrivateKey) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = WirePrivateKey{}
	if err := dec.StartValue(vdlTypeStruct5); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != vdlTypeStruct5 {
			index = vdlTypeStruct5.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Blessing = value
			}
		case 1:
			if err := x.Params.VDLRead(dec); err != nil {
				return err
			}
		case 2:
			if err := vdlReadAnonList2(dec, &x.Keys); err != nil {
				return err
			}
		}
	}
}

func vdlReadAnonList2(dec vdl.Decoder, x *[][]byte) error {
	if err := dec.StartValue(vdlTypeList6); err != nil {
		return err
	}
	if len := dec.LenHint(); len > 0 {
		*x = make([][]byte, 0, len)
	} else {
		*x = nil
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		default:
			var elem []byte
			if err := dec.ReadValueBytes(-1, &elem); err != nil {
				return err
			}
			*x = append(*x, elem)
		}
	}
}

// Error definitions
// =================

var (
	ErrInternal           = verror.NewIDAction("v.io/x/ref/lib/security/bcrypter.Internal", verror.NoRetry)
	ErrNoParams           = verror.NewIDAction("v.io/x/ref/lib/security/bcrypter.NoParams", verror.NoRetry)
	ErrPrivateKeyNotFound = verror.NewIDAction("v.io/x/ref/lib/security/bcrypter.PrivateKeyNotFound", verror.NoRetry)
	ErrInvalidPrivateKey  = verror.NewIDAction("v.io/x/ref/lib/security/bcrypter.InvalidPrivateKey", verror.NoRetry)
)

// ErrorfInternal calls ErrInternal.Errorf with the supplied arguments.
func ErrorfInternal(ctx *context.T, format string, err error) error {
	return ErrInternal.Errorf(ctx, format, err)
}

// MessageInternal calls ErrInternal.Message with the supplied arguments.
func MessageInternal(ctx *context.T, message string, err error) error {
	return ErrInternal.Message(ctx, message, err)
}

// ParamsErrInternal extracts the expected parameters from the error's ParameterList.
func ParamsErrInternal(argumentError error) (verrorComponent string, verrorOperation string, err error, returnErr error) {
	params := verror.Params(argumentError)
	if params == nil {
		returnErr = fmt.Errorf("no parameters found in: %T: %v", argumentError, argumentError)
		return
	}
	iter := &paramListIterator{params: params, max: len(params)}

	if verrorComponent, verrorOperation, returnErr = iter.preamble(); returnErr != nil {
		return
	}

	var (
		tmp interface{}
		ok  bool
	)
	tmp, returnErr = iter.next()
	if err, ok = tmp.(error); !ok {
		if returnErr != nil {
			return
		}
		returnErr = fmt.Errorf("parameter list contains the wrong type for return value err, has %T and not error", tmp)
		return
	}

	return
}

// ErrorfNoParams calls ErrNoParams.Errorf with the supplied arguments.
func ErrorfNoParams(ctx *context.T, format string, pattern security.BlessingPattern) error {
	return ErrNoParams.Errorf(ctx, format, pattern)
}

// MessageNoParams calls ErrNoParams.Message with the supplied arguments.
func MessageNoParams(ctx *context.T, message string, pattern security.BlessingPattern) error {
	return ErrNoParams.Message(ctx, message, pattern)
}

// ParamsErrNoParams extracts the expected parameters from the error's ParameterList.
func ParamsErrNoParams(argumentError error) (verrorComponent string, verrorOperation string, pattern security.BlessingPattern, returnErr error) {
	params := verror.Params(argumentError)
	if params == nil {
		returnErr = fmt.Errorf("no parameters found in: %T: %v", argumentError, argumentError)
		return
	}
	iter := &paramListIterator{params: params, max: len(params)}

	if verrorComponent, verrorOperation, returnErr = iter.preamble(); returnErr != nil {
		return
	}

	var (
		tmp interface{}
		ok  bool
	)
	tmp, returnErr = iter.next()
	if pattern, ok = tmp.(security.BlessingPattern); !ok {
		if returnErr != nil {
			return
		}
		returnErr = fmt.Errorf("parameter list contains the wrong type for return value pattern, has %T and not security.BlessingPattern", tmp)
		return
	}

	return
}

// ErrorfPrivateKeyNotFound calls ErrPrivateKeyNotFound.Errorf with the supplied arguments.
func ErrorfPrivateKeyNotFound(ctx *context.T, format string) error {
	return ErrPrivateKeyNotFound.Errorf(ctx, format)
}

// MessagePrivateKeyNotFound calls ErrPrivateKeyNotFound.Message with the supplied arguments.
func MessagePrivateKeyNotFound(ctx *context.T, message string) error {
	return ErrPrivateKeyNotFound.Message(ctx, message)
}

// ParamsErrPrivateKeyNotFound extracts the expected parameters from the error's ParameterList.
func ParamsErrPrivateKeyNotFound(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := verror.Params(argumentError)
	if params == nil {
		returnErr = fmt.Errorf("no parameters found in: %T: %v", argumentError, argumentError)
		return
	}
	iter := &paramListIterator{params: params, max: len(params)}

	if verrorComponent, verrorOperation, returnErr = iter.preamble(); returnErr != nil {
		return
	}

	return
}

// ErrorfInvalidPrivateKey calls ErrInvalidPrivateKey.Errorf with the supplied arguments.
func ErrorfInvalidPrivateKey(ctx *context.T, format string, err error) error {
	return ErrInvalidPrivateKey.Errorf(ctx, format, err)
}

// MessageInvalidPrivateKey calls ErrInvalidPrivateKey.Message with the supplied arguments.
func MessageInvalidPrivateKey(ctx *context.T, message string, err error) error {
	return ErrInvalidPrivateKey.Message(ctx, message, err)
}

// ParamsErrInvalidPrivateKey extracts the expected parameters from the error's ParameterList.
func ParamsErrInvalidPrivateKey(argumentError error) (verrorComponent string, verrorOperation string, err error, returnErr error) {
	params := verror.Params(argumentError)
	if params == nil {
		returnErr = fmt.Errorf("no parameters found in: %T: %v", argumentError, argumentError)
		return
	}
	iter := &paramListIterator{params: params, max: len(params)}

	if verrorComponent, verrorOperation, returnErr = iter.preamble(); returnErr != nil {
		return
	}

	var (
		tmp interface{}
		ok  bool
	)
	tmp, returnErr = iter.next()
	if err, ok = tmp.(error); !ok {
		if returnErr != nil {
			return
		}
		returnErr = fmt.Errorf("parameter list contains the wrong type for return value err, has %T and not error", tmp)
		return
	}

	return
}

type paramListIterator struct {
	err      error
	idx, max int
	params   []interface{}
}

func (pl *paramListIterator) next() (interface{}, error) {
	if pl.err != nil {
		return nil, pl.err
	}
	if pl.idx+1 > pl.max {
		pl.err = fmt.Errorf("too few parameters: have %v", pl.max)
		return nil, pl.err
	}
	pl.idx++
	return pl.params[pl.idx-1], nil
}

func (pl *paramListIterator) preamble() (component, operation string, err error) {
	var tmp interface{}
	if tmp, err = pl.next(); err != nil {
		return
	}
	var ok bool
	if component, ok = tmp.(string); !ok {
		return "", "", fmt.Errorf("ParamList[0]: component name is not a string: %T", tmp)
	}
	if tmp, err = pl.next(); err != nil {
		return
	}
	if operation, ok = tmp.(string); !ok {
		return "", "", fmt.Errorf("ParamList[1]: operation name is not a string: %T", tmp)
	}
	return
}

// Hold type definitions in package-level variables, for better performance.
//nolint:unused
var (
	vdlTypeStruct1 *vdl.Type
	vdlTypeMap2    *vdl.Type
	vdlTypeList3   *vdl.Type
	vdlTypeStruct4 *vdl.Type
	vdlTypeStruct5 *vdl.Type
	vdlTypeList6   *vdl.Type
)

var initializeVDLCalled bool

// initializeVDL performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = initializeVDL()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func initializeVDL() struct{} {
	if initializeVDLCalled {
		return struct{}{}
	}
	initializeVDLCalled = true

	// Register types.
	vdl.Register((*WireCiphertext)(nil))
	vdl.Register((*WireParams)(nil))
	vdl.Register((*WirePrivateKey)(nil))

	// Initialize type definitions.
	vdlTypeStruct1 = vdl.TypeOf((*WireCiphertext)(nil)).Elem()
	vdlTypeMap2 = vdl.TypeOf((*map[string][]byte)(nil))
	vdlTypeList3 = vdl.TypeOf((*[]byte)(nil))
	vdlTypeStruct4 = vdl.TypeOf((*WireParams)(nil)).Elem()
	vdlTypeStruct5 = vdl.TypeOf((*WirePrivateKey)(nil)).Elem()
	vdlTypeList6 = vdl.TypeOf((*[][]byte)(nil))

	return struct{}{}
}
