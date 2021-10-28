// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: discovery

//nolint:revive
package discovery

import (
	"fmt"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

var initializeVDLCalled = false
var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

// Hold type definitions in package-level variables, for better performance.
// Declare and initialize with default values here so that the initializeVDL
// method will be considered ready to initialize before any of the type
// definitions that appear below.
//nolint:unused
var (
	vdlTypeList1   *vdl.Type = nil
	vdlTypeInt322  *vdl.Type = nil
	vdlTypeList3   *vdl.Type = nil
	vdlTypeByte4   *vdl.Type = nil
	vdlTypeArray5  *vdl.Type = nil
	vdlTypeStruct6 *vdl.Type = nil
	vdlTypeStruct7 *vdl.Type = nil
	vdlTypeList8   *vdl.Type = nil
	vdlTypeList9   *vdl.Type = nil
)

// Type definitions
// ================
type Uuid []byte

func (Uuid) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.Uuid"`
}) {
}

func (x Uuid) VDLIsZero() bool { //nolint:gocyclo
	return len(x) == 0
}

func (x Uuid) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueBytes(vdlTypeList1, []byte(x)); err != nil {
		return err
	}
	return nil
}

func (x *Uuid) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	var bytes []byte
	if err := dec.ReadValueBytes(-1, &bytes); err != nil {
		return err
	}
	*x = bytes
	return nil
}

type EncryptionAlgorithm int32

func (EncryptionAlgorithm) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.EncryptionAlgorithm"`
}) {
}

func (x EncryptionAlgorithm) VDLIsZero() bool { //nolint:gocyclo
	return x == 0
}

func (x EncryptionAlgorithm) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueInt(vdlTypeInt322, int64(x)); err != nil {
		return err
	}
	return nil
}

func (x *EncryptionAlgorithm) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	switch value, err := dec.ReadValueInt(32); {
	case err != nil:
		return err
	default:
		*x = EncryptionAlgorithm(value)
	}
	return nil
}

type EncryptionKey []byte

func (EncryptionKey) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.EncryptionKey"`
}) {
}

func (x EncryptionKey) VDLIsZero() bool { //nolint:gocyclo
	return len(x) == 0
}

func (x EncryptionKey) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueBytes(vdlTypeList3, []byte(x)); err != nil {
		return err
	}
	return nil
}

func (x *EncryptionKey) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	var bytes []byte
	if err := dec.ReadValueBytes(-1, &bytes); err != nil {
		return err
	}
	*x = bytes
	return nil
}

type AdStatus byte

func (AdStatus) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.AdStatus"`
}) {
}

func (x AdStatus) VDLIsZero() bool { //nolint:gocyclo
	return x == 0
}

func (x AdStatus) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueUint(vdlTypeByte4, uint64(x)); err != nil {
		return err
	}
	return nil
}

func (x *AdStatus) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	switch value, err := dec.ReadValueUint(8); {
	case err != nil:
		return err
	default:
		*x = AdStatus(value)
	}
	return nil
}

// An AdHash is a hash of an advertisement.
type AdHash [8]byte

func (AdHash) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.AdHash"`
}) {
}

func (x AdHash) VDLIsZero() bool { //nolint:gocyclo
	return x == AdHash{}
}

func (x AdHash) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueBytes(vdlTypeArray5, x[:]); err != nil {
		return err
	}
	return nil
}

func (x *AdHash) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	bytes := x[:]
	if err := dec.ReadValueBytes(8, &bytes); err != nil {
		return err
	}
	return nil
}

// AdInfo represents advertisement information for discovery.
type AdInfo struct {
	Ad discovery.Advertisement
	// Type of encryption applied to the advertisement so that it can
	// only be decoded by authorized principals.
	EncryptionAlgorithm EncryptionAlgorithm
	// If the advertisement is encrypted, then the data required to
	// decrypt it. The format of this data is a function of the algorithm.
	EncryptionKeys []EncryptionKey
	// Hash of the current advertisement. This does not include the fields below.
	Hash AdHash
	// Unix time in nanoseconds at which the advertisement was created.
	TimestampNs int64
	// The addresses (vanadium object names) that the advertisement directory service
	// is served on. See directory.vdl.
	DirAddrs []string
	// Status of the current advertisement. Valid for scanned advertisements.
	Status AdStatus
	// TODO(jhahn): Add proximity.
	// TODO(jhahn): Use proximity for Lost.
	Lost bool
}

func (AdInfo) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.AdInfo"`
}) {
}

func (x AdInfo) VDLIsZero() bool { //nolint:gocyclo
	if !x.Ad.VDLIsZero() {
		return false
	}
	if x.EncryptionAlgorithm != 0 {
		return false
	}
	if len(x.EncryptionKeys) != 0 {
		return false
	}
	if x.Hash != (AdHash{}) {
		return false
	}
	if x.TimestampNs != 0 {
		return false
	}
	if len(x.DirAddrs) != 0 {
		return false
	}
	if x.Status != 0 {
		return false
	}
	if x.Lost {
		return false
	}
	return true
}

func (x AdInfo) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct6); err != nil {
		return err
	}
	if !x.Ad.VDLIsZero() {
		if err := enc.NextField(0); err != nil {
			return err
		}
		if err := x.Ad.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.EncryptionAlgorithm != 0 {
		if err := enc.NextFieldValueInt(1, vdlTypeInt322, int64(x.EncryptionAlgorithm)); err != nil {
			return err
		}
	}
	if len(x.EncryptionKeys) != 0 {
		if err := enc.NextField(2); err != nil {
			return err
		}
		if err := vdlWriteAnonList1(enc, x.EncryptionKeys); err != nil {
			return err
		}
	}
	if x.Hash != (AdHash{}) {
		if err := enc.NextFieldValueBytes(3, vdlTypeArray5, x.Hash[:]); err != nil {
			return err
		}
	}
	if x.TimestampNs != 0 {
		if err := enc.NextFieldValueInt(4, vdl.Int64Type, x.TimestampNs); err != nil {
			return err
		}
	}
	if len(x.DirAddrs) != 0 {
		if err := enc.NextField(5); err != nil {
			return err
		}
		if err := vdlWriteAnonList2(enc, x.DirAddrs); err != nil {
			return err
		}
	}
	if x.Status != 0 {
		if err := enc.NextFieldValueUint(6, vdlTypeByte4, uint64(x.Status)); err != nil {
			return err
		}
	}
	if x.Lost {
		if err := enc.NextFieldValueBool(7, vdl.BoolType, x.Lost); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func vdlWriteAnonList1(enc vdl.Encoder, x []EncryptionKey) error {
	if err := enc.StartValue(vdlTypeList8); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for _, elem := range x {
		if err := enc.NextEntryValueBytes(vdlTypeList3, []byte(elem)); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func vdlWriteAnonList2(enc vdl.Encoder, x []string) error {
	if err := enc.StartValue(vdlTypeList9); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for _, elem := range x {
		if err := enc.NextEntryValueString(vdl.StringType, elem); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *AdInfo) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = AdInfo{}
	if err := dec.StartValue(vdlTypeStruct6); err != nil {
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
		if decType != vdlTypeStruct6 {
			index = vdlTypeStruct6.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			if err := x.Ad.VDLRead(dec); err != nil {
				return err
			}
		case 1:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.EncryptionAlgorithm = EncryptionAlgorithm(value)
			}
		case 2:
			if err := vdlReadAnonList1(dec, &x.EncryptionKeys); err != nil {
				return err
			}
		case 3:
			bytes := x.Hash[:]
			if err := dec.ReadValueBytes(8, &bytes); err != nil {
				return err
			}
		case 4:
			switch value, err := dec.ReadValueInt(64); {
			case err != nil:
				return err
			default:
				x.TimestampNs = value
			}
		case 5:
			if err := vdlReadAnonList2(dec, &x.DirAddrs); err != nil {
				return err
			}
		case 6:
			switch value, err := dec.ReadValueUint(8); {
			case err != nil:
				return err
			default:
				x.Status = AdStatus(value)
			}
		case 7:
			switch value, err := dec.ReadValueBool(); {
			case err != nil:
				return err
			default:
				x.Lost = value
			}
		}
	}
}

func vdlReadAnonList1(dec vdl.Decoder, x *[]EncryptionKey) error {
	if err := dec.StartValue(vdlTypeList8); err != nil {
		return err
	}
	if len := dec.LenHint(); len > 0 {
		*x = make([]EncryptionKey, 0, len)
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
			var elem EncryptionKey
			var bytes []byte
			if err := dec.ReadValueBytes(-1, &bytes); err != nil {
				return err
			}
			elem = bytes
			*x = append(*x, elem)
		}
	}
}

func vdlReadAnonList2(dec vdl.Decoder, x *[]string) error {
	if err := dec.StartValue(vdlTypeList9); err != nil {
		return err
	}
	if len := dec.LenHint(); len > 0 {
		*x = make([]string, 0, len)
	} else {
		*x = nil
	}
	for {
		switch done, elem, err := dec.NextEntryValueString(); {
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		default:
			*x = append(*x, elem)
		}
	}
}

// Const definitions
// =================

const NoEncryption = EncryptionAlgorithm(0)
const TestEncryption = EncryptionAlgorithm(1)
const IbeEncryption = EncryptionAlgorithm(2)
const AdReady = AdStatus(0)          // All information is available
const AdNotReady = AdStatus(1)       // Not all information is available for querying against it
const AdPartiallyReady = AdStatus(2) // All information except attachments is available

// Error definitions
// =================

var (
	ErrAdvertisementNotFound  = verror.NewIDAction("v.io/x/ref/lib/discovery.AdvertisementNotFound", verror.NoRetry)
	ErrAlreadyBeingAdvertised = verror.NewIDAction("v.io/x/ref/lib/discovery.AlreadyBeingAdvertised", verror.NoRetry)
	ErrBadAdvertisement       = verror.NewIDAction("v.io/x/ref/lib/discovery.BadAdvertisement", verror.NoRetry)
	ErrBadQuery               = verror.NewIDAction("v.io/x/ref/lib/discovery.BadQuery", verror.NoRetry)
	ErrDiscoveryClosed        = verror.NewIDAction("v.io/x/ref/lib/discovery.DiscoveryClosed", verror.NoRetry)
	ErrNoDiscoveryPlugin      = verror.NewIDAction("v.io/x/ref/lib/discovery.NoDiscoveryPlugin", verror.NoRetry)
	ErrTooManyPlugins         = verror.NewIDAction("v.io/x/ref/lib/discovery.TooManyPlugins", verror.NoRetry)
)

// ErrorfAdvertisementNotFound calls ErrAdvertisementNotFound.Errorf with the supplied arguments.
func ErrorfAdvertisementNotFound(ctx *context.T, format string, id discovery.AdId) error {
	return ErrAdvertisementNotFound.Errorf(ctx, format, id)
}

// MessageAdvertisementNotFound calls ErrAdvertisementNotFound.Message with the supplied arguments.
func MessageAdvertisementNotFound(ctx *context.T, message string, id discovery.AdId) error {
	return ErrAdvertisementNotFound.Message(ctx, message, id)
}

// ParamsErrAdvertisementNotFound extracts the expected parameters from the error's ParameterList.
func ParamsErrAdvertisementNotFound(argumentError error) (verrorComponent string, verrorOperation string, id discovery.AdId, returnErr error) {
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
	if id, ok = tmp.(discovery.AdId); !ok {
		if returnErr != nil {
			return
		}
		returnErr = fmt.Errorf("parameter list contains the wrong type for return value id, has %T and not discovery.AdId", tmp)
		return
	}

	return
}

// ErrorfAlreadyBeingAdvertised calls ErrAlreadyBeingAdvertised.Errorf with the supplied arguments.
func ErrorfAlreadyBeingAdvertised(ctx *context.T, format string, id discovery.AdId) error {
	return ErrAlreadyBeingAdvertised.Errorf(ctx, format, id)
}

// MessageAlreadyBeingAdvertised calls ErrAlreadyBeingAdvertised.Message with the supplied arguments.
func MessageAlreadyBeingAdvertised(ctx *context.T, message string, id discovery.AdId) error {
	return ErrAlreadyBeingAdvertised.Message(ctx, message, id)
}

// ParamsErrAlreadyBeingAdvertised extracts the expected parameters from the error's ParameterList.
func ParamsErrAlreadyBeingAdvertised(argumentError error) (verrorComponent string, verrorOperation string, id discovery.AdId, returnErr error) {
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
	if id, ok = tmp.(discovery.AdId); !ok {
		if returnErr != nil {
			return
		}
		returnErr = fmt.Errorf("parameter list contains the wrong type for return value id, has %T and not discovery.AdId", tmp)
		return
	}

	return
}

// ErrorfBadAdvertisement calls ErrBadAdvertisement.Errorf with the supplied arguments.
func ErrorfBadAdvertisement(ctx *context.T, format string, err error) error {
	return ErrBadAdvertisement.Errorf(ctx, format, err)
}

// MessageBadAdvertisement calls ErrBadAdvertisement.Message with the supplied arguments.
func MessageBadAdvertisement(ctx *context.T, message string, err error) error {
	return ErrBadAdvertisement.Message(ctx, message, err)
}

// ParamsErrBadAdvertisement extracts the expected parameters from the error's ParameterList.
func ParamsErrBadAdvertisement(argumentError error) (verrorComponent string, verrorOperation string, err error, returnErr error) {
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

// ErrorfBadQuery calls ErrBadQuery.Errorf with the supplied arguments.
func ErrorfBadQuery(ctx *context.T, format string, err error) error {
	return ErrBadQuery.Errorf(ctx, format, err)
}

// MessageBadQuery calls ErrBadQuery.Message with the supplied arguments.
func MessageBadQuery(ctx *context.T, message string, err error) error {
	return ErrBadQuery.Message(ctx, message, err)
}

// ParamsErrBadQuery extracts the expected parameters from the error's ParameterList.
func ParamsErrBadQuery(argumentError error) (verrorComponent string, verrorOperation string, err error, returnErr error) {
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

// ErrorfDiscoveryClosed calls ErrDiscoveryClosed.Errorf with the supplied arguments.
func ErrorfDiscoveryClosed(ctx *context.T, format string) error {
	return ErrDiscoveryClosed.Errorf(ctx, format)
}

// MessageDiscoveryClosed calls ErrDiscoveryClosed.Message with the supplied arguments.
func MessageDiscoveryClosed(ctx *context.T, message string) error {
	return ErrDiscoveryClosed.Message(ctx, message)
}

// ParamsErrDiscoveryClosed extracts the expected parameters from the error's ParameterList.
func ParamsErrDiscoveryClosed(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
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

// ErrorfNoDiscoveryPlugin calls ErrNoDiscoveryPlugin.Errorf with the supplied arguments.
func ErrorfNoDiscoveryPlugin(ctx *context.T, format string) error {
	return ErrNoDiscoveryPlugin.Errorf(ctx, format)
}

// MessageNoDiscoveryPlugin calls ErrNoDiscoveryPlugin.Message with the supplied arguments.
func MessageNoDiscoveryPlugin(ctx *context.T, message string) error {
	return ErrNoDiscoveryPlugin.Message(ctx, message)
}

// ParamsErrNoDiscoveryPlugin extracts the expected parameters from the error's ParameterList.
func ParamsErrNoDiscoveryPlugin(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
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

// ErrorfTooManyPlugins calls ErrTooManyPlugins.Errorf with the supplied arguments.
func ErrorfTooManyPlugins(ctx *context.T, format string, actual int32, limit int32) error {
	return ErrTooManyPlugins.Errorf(ctx, format, actual, limit)
}

// MessageTooManyPlugins calls ErrTooManyPlugins.Message with the supplied arguments.
func MessageTooManyPlugins(ctx *context.T, message string, actual int32, limit int32) error {
	return ErrTooManyPlugins.Message(ctx, message, actual, limit)
}

// ParamsErrTooManyPlugins extracts the expected parameters from the error's ParameterList.
func ParamsErrTooManyPlugins(argumentError error) (verrorComponent string, verrorOperation string, actual int32, limit int32, returnErr error) {
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
	if actual, ok = tmp.(int32); !ok {
		if returnErr != nil {
			return
		}
		returnErr = fmt.Errorf("parameter list contains the wrong type for return value actual, has %T and not int32", tmp)
		return
	}
	tmp, returnErr = iter.next()
	if limit, ok = tmp.(int32); !ok {
		if returnErr != nil {
			return
		}
		returnErr = fmt.Errorf("parameter list contains the wrong type for return value limit, has %T and not int32", tmp)
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

// Interface definitions
// =====================

// DirectoryClientMethods is the client interface
// containing Directory methods.
//
// Directory is the interface for advertisement directory service.
type DirectoryClientMethods interface {
	// Lookup returns the advertisement of the given service instance.
	//
	// The returned advertisement may not include all attachments.
	Lookup(_ *context.T, id discovery.AdId, _ ...rpc.CallOpt) (AdInfo, error)
	// GetAttachment returns the named attachment. Accessing non-existent attachment
	// is not an error - nil data is returned if not found.
	//
	// TODO(jhahn): Consider to return an error if not found.
	GetAttachment(_ *context.T, id discovery.AdId, name string, _ ...rpc.CallOpt) ([]byte, error)
}

// DirectoryClientStub embeds DirectoryClientMethods and is a
// placeholder for additional management operations.
type DirectoryClientStub interface {
	DirectoryClientMethods
}

// DirectoryClient returns a client stub for Directory.
func DirectoryClient(name string) DirectoryClientStub {
	return implDirectoryClientStub{name}
}

type implDirectoryClientStub struct {
	name string
}

func (c implDirectoryClientStub) Lookup(ctx *context.T, i0 discovery.AdId, opts ...rpc.CallOpt) (o0 AdInfo, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Lookup", []interface{}{i0}, []interface{}{&o0}, opts...)
	return
}

func (c implDirectoryClientStub) GetAttachment(ctx *context.T, i0 discovery.AdId, i1 string, opts ...rpc.CallOpt) (o0 []byte, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "GetAttachment", []interface{}{i0, i1}, []interface{}{&o0}, opts...)
	return
}

// DirectoryServerMethods is the interface a server writer
// implements for Directory.
//
// Directory is the interface for advertisement directory service.
type DirectoryServerMethods interface {
	// Lookup returns the advertisement of the given service instance.
	//
	// The returned advertisement may not include all attachments.
	Lookup(_ *context.T, _ rpc.ServerCall, id discovery.AdId) (AdInfo, error)
	// GetAttachment returns the named attachment. Accessing non-existent attachment
	// is not an error - nil data is returned if not found.
	//
	// TODO(jhahn): Consider to return an error if not found.
	GetAttachment(_ *context.T, _ rpc.ServerCall, id discovery.AdId, name string) ([]byte, error)
}

// DirectoryServerStubMethods is the server interface containing
// Directory methods, as expected by rpc.Server.
// There is no difference between this interface and DirectoryServerMethods
// since there are no streaming methods.
type DirectoryServerStubMethods DirectoryServerMethods

// DirectoryServerStub adds universal methods to DirectoryServerStubMethods.
type DirectoryServerStub interface {
	DirectoryServerStubMethods
	// DescribeInterfaces the Directory interfaces.
	Describe__() []rpc.InterfaceDesc
}

// DirectoryServer returns a server stub for Directory.
// It converts an implementation of DirectoryServerMethods into
// an object that may be used by rpc.Server.
func DirectoryServer(impl DirectoryServerMethods) DirectoryServerStub {
	stub := implDirectoryServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := rpc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := rpc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implDirectoryServerStub struct {
	impl DirectoryServerMethods
	gs   *rpc.GlobState
}

func (s implDirectoryServerStub) Lookup(ctx *context.T, call rpc.ServerCall, i0 discovery.AdId) (AdInfo, error) {
	return s.impl.Lookup(ctx, call, i0)
}

func (s implDirectoryServerStub) GetAttachment(ctx *context.T, call rpc.ServerCall, i0 discovery.AdId, i1 string) ([]byte, error) {
	return s.impl.GetAttachment(ctx, call, i0, i1)
}

func (s implDirectoryServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implDirectoryServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{DirectoryDesc}
}

// DirectoryDesc describes the Directory interface.
var DirectoryDesc rpc.InterfaceDesc = descDirectory

// descDirectory hides the desc to keep godoc clean.
var descDirectory = rpc.InterfaceDesc{
	Name:    "Directory",
	PkgPath: "v.io/x/ref/lib/discovery",
	Doc:     "// Directory is the interface for advertisement directory service.",
	Methods: []rpc.MethodDesc{
		{
			Name: "Lookup",
			Doc:  "// Lookup returns the advertisement of the given service instance.\n//\n// The returned advertisement may not include all attachments.",
			InArgs: []rpc.ArgDesc{
				{Name: "id", Doc: ``}, // discovery.AdId
			},
			OutArgs: []rpc.ArgDesc{
				{Name: "", Doc: ``}, // AdInfo
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
		{
			Name: "GetAttachment",
			Doc:  "// GetAttachment returns the named attachment. Accessing non-existent attachment\n// is not an error - nil data is returned if not found.\n//\n// TODO(jhahn): Consider to return an error if not found.",
			InArgs: []rpc.ArgDesc{
				{Name: "id", Doc: ``},   // discovery.AdId
				{Name: "name", Doc: ``}, // string
			},
			OutArgs: []rpc.ArgDesc{
				{Name: "", Doc: ``}, // []byte
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
	},
}

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
	vdl.Register((*Uuid)(nil))
	vdl.Register((*EncryptionAlgorithm)(nil))
	vdl.Register((*EncryptionKey)(nil))
	vdl.Register((*AdStatus)(nil))
	vdl.Register((*AdHash)(nil))
	vdl.Register((*AdInfo)(nil))

	// Initialize type definitions.
	vdlTypeList1 = vdl.TypeOf((*Uuid)(nil))
	vdlTypeInt322 = vdl.TypeOf((*EncryptionAlgorithm)(nil))
	vdlTypeList3 = vdl.TypeOf((*EncryptionKey)(nil))
	vdlTypeByte4 = vdl.TypeOf((*AdStatus)(nil))
	vdlTypeArray5 = vdl.TypeOf((*AdHash)(nil))
	vdlTypeStruct6 = vdl.TypeOf((*AdInfo)(nil)).Elem()
	vdlTypeStruct7 = vdl.TypeOf((*discovery.Advertisement)(nil)).Elem()
	vdlTypeList8 = vdl.TypeOf((*[]EncryptionKey)(nil))
	vdlTypeList9 = vdl.TypeOf((*[]string)(nil))

	return struct{}{}
}
