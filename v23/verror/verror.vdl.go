// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: verror

//nolint:golint
package verror

import (
	"fmt"

	"v.io/v23/context"
)

var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

// Error definitions
// =================

var (

	// ErrUnknown means the error has no known Id.  A more specific error should
	// always be used, if possible.  Unknown is typically only used when
	// automatically converting errors that do not contain an Id.
	ErrUnknown = NewIDAction("v.io/v23/verror.Unknown", NoRetry)
	// ErrInternal means an internal error has occurred.  A more specific error
	// should always be used, if possible.
	ErrInternal = NewIDAction("v.io/v23/verror.Internal", NoRetry)
	// ErrNotImplemented means that the request type is valid but that the method to
	// handle the request has not been implemented.
	ErrNotImplemented = NewIDAction("v.io/v23/verror.NotImplemented", NoRetry)
	// ErrEndOfFile means the end-of-file has been reached; more generally, no more
	// input data is available.
	ErrEndOfFile = NewIDAction("v.io/v23/verror.EndOfFile", NoRetry)
	// ErrBadArg means the arguments to an operation are invalid or incorrectly
	// formatted.
	ErrBadArg = NewIDAction("v.io/v23/verror.BadArg", NoRetry)
	// ErrBadState means an operation was attempted on an object while the object was
	// in an incompatible state.
	ErrBadState = NewIDAction("v.io/v23/verror.BadState", NoRetry)
	// ErrBadVersion means the version presented by the client (e.g. to a service
	// that supports content-hash-based caching or atomic read-modify-write) was
	// out of date or otherwise invalid, likely because some other request caused
	// the version at the server to change. The client should get a fresh version
	// and try again.
	ErrBadVersion = NewIDAction("v.io/v23/verror.BadVersion", NoRetry)
	// ErrExist means that the requested item already exists; typically returned when
	// an attempt to create an item fails because it already exists.
	ErrExist = NewIDAction("v.io/v23/verror.Exist", NoRetry)
	// ErrNoExist means that the requested item does not exist; typically returned
	// when an attempt to lookup an item fails because it does not exist.
	ErrNoExist       = NewIDAction("v.io/v23/verror.NoExist", NoRetry)
	ErrUnknownMethod = NewIDAction("v.io/v23/verror.UnknownMethod", NoRetry)
	ErrUnknownSuffix = NewIDAction("v.io/v23/verror.UnknownSuffix", NoRetry)
	// ErrNoExistOrNoAccess means that either the requested item does not exist, or
	// is inaccessible.  Typically returned when the distinction between existence
	// and inaccessiblity should be hidden to preserve privacy.
	ErrNoExistOrNoAccess = NewIDAction("v.io/v23/verror.NoExistOrNoAccess", NoRetry)
	// ErrNoServers means a name was resolved to unusable or inaccessible servers.
	ErrNoServers = NewIDAction("v.io/v23/verror.NoServers", RetryRefetch)
	// ErrNoAccess means the server does not authorize the client for access.
	ErrNoAccess = NewIDAction("v.io/v23/verror.NoAccess", RetryRefetch)
	// ErrNotTrusted means the client does not trust the server.
	ErrNotTrusted = NewIDAction("v.io/v23/verror.NotTrusted", RetryRefetch)
	// ErrAborted means that an operation was not completed because it was aborted by
	// the receiver.  A more specific error should be used if it would help the
	// caller decide how to proceed.
	ErrAborted = NewIDAction("v.io/v23/verror.Aborted", NoRetry)
	// ErrBadProtocol means that an operation was not completed because of a protocol
	// or codec error.
	ErrBadProtocol = NewIDAction("v.io/v23/verror.BadProtocol", NoRetry)
	// ErrCanceled means that an operation was not completed because it was
	// explicitly cancelled by the caller.
	ErrCanceled = NewIDAction("v.io/v23/verror.Canceled", NoRetry)
	// ErrTimeout means that an operation was not completed before the time deadline
	// for the operation.
	ErrTimeout = NewIDAction("v.io/v23/verror.Timeout", NoRetry)
)

// ErrorfUnknown calls ErrUnknown.Errorf with the supplied arguments.
func ErrorfUnknown(ctx *context.T, format string) error {
	return ErrUnknown.Errorf(ctx, format)
}

// MessageUnknown calls ErrUnknown.Message with the supplied arguments.
func MessageUnknown(ctx *context.T, message string) error {
	return ErrUnknown.Message(ctx, message)
}

// ParamsErrUnknown extracts the expected parameters from the error's ParameterList.
func ParamsErrUnknown(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfInternal calls ErrInternal.Errorf with the supplied arguments.
func ErrorfInternal(ctx *context.T, format string) error {
	return ErrInternal.Errorf(ctx, format)
}

// MessageInternal calls ErrInternal.Message with the supplied arguments.
func MessageInternal(ctx *context.T, message string) error {
	return ErrInternal.Message(ctx, message)
}

// ParamsErrInternal extracts the expected parameters from the error's ParameterList.
func ParamsErrInternal(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfNotImplemented calls ErrNotImplemented.Errorf with the supplied arguments.
func ErrorfNotImplemented(ctx *context.T, format string) error {
	return ErrNotImplemented.Errorf(ctx, format)
}

// MessageNotImplemented calls ErrNotImplemented.Message with the supplied arguments.
func MessageNotImplemented(ctx *context.T, message string) error {
	return ErrNotImplemented.Message(ctx, message)
}

// ParamsErrNotImplemented extracts the expected parameters from the error's ParameterList.
func ParamsErrNotImplemented(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfEndOfFile calls ErrEndOfFile.Errorf with the supplied arguments.
func ErrorfEndOfFile(ctx *context.T, format string) error {
	return ErrEndOfFile.Errorf(ctx, format)
}

// MessageEndOfFile calls ErrEndOfFile.Message with the supplied arguments.
func MessageEndOfFile(ctx *context.T, message string) error {
	return ErrEndOfFile.Message(ctx, message)
}

// ParamsErrEndOfFile extracts the expected parameters from the error's ParameterList.
func ParamsErrEndOfFile(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfBadArg calls ErrBadArg.Errorf with the supplied arguments.
func ErrorfBadArg(ctx *context.T, format string) error {
	return ErrBadArg.Errorf(ctx, format)
}

// MessageBadArg calls ErrBadArg.Message with the supplied arguments.
func MessageBadArg(ctx *context.T, message string) error {
	return ErrBadArg.Message(ctx, message)
}

// ParamsErrBadArg extracts the expected parameters from the error's ParameterList.
func ParamsErrBadArg(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfBadState calls ErrBadState.Errorf with the supplied arguments.
func ErrorfBadState(ctx *context.T, format string) error {
	return ErrBadState.Errorf(ctx, format)
}

// MessageBadState calls ErrBadState.Message with the supplied arguments.
func MessageBadState(ctx *context.T, message string) error {
	return ErrBadState.Message(ctx, message)
}

// ParamsErrBadState extracts the expected parameters from the error's ParameterList.
func ParamsErrBadState(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfBadVersion calls ErrBadVersion.Errorf with the supplied arguments.
func ErrorfBadVersion(ctx *context.T, format string) error {
	return ErrBadVersion.Errorf(ctx, format)
}

// MessageBadVersion calls ErrBadVersion.Message with the supplied arguments.
func MessageBadVersion(ctx *context.T, message string) error {
	return ErrBadVersion.Message(ctx, message)
}

// ParamsErrBadVersion extracts the expected parameters from the error's ParameterList.
func ParamsErrBadVersion(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfExist calls ErrExist.Errorf with the supplied arguments.
func ErrorfExist(ctx *context.T, format string) error {
	return ErrExist.Errorf(ctx, format)
}

// MessageExist calls ErrExist.Message with the supplied arguments.
func MessageExist(ctx *context.T, message string) error {
	return ErrExist.Message(ctx, message)
}

// ParamsErrExist extracts the expected parameters from the error's ParameterList.
func ParamsErrExist(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfNoExist calls ErrNoExist.Errorf with the supplied arguments.
func ErrorfNoExist(ctx *context.T, format string) error {
	return ErrNoExist.Errorf(ctx, format)
}

// MessageNoExist calls ErrNoExist.Message with the supplied arguments.
func MessageNoExist(ctx *context.T, message string) error {
	return ErrNoExist.Message(ctx, message)
}

// ParamsErrNoExist extracts the expected parameters from the error's ParameterList.
func ParamsErrNoExist(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfUnknownMethod calls ErrUnknownMethod.Errorf with the supplied arguments.
func ErrorfUnknownMethod(ctx *context.T, format string) error {
	return ErrUnknownMethod.Errorf(ctx, format)
}

// MessageUnknownMethod calls ErrUnknownMethod.Message with the supplied arguments.
func MessageUnknownMethod(ctx *context.T, message string) error {
	return ErrUnknownMethod.Message(ctx, message)
}

// ParamsErrUnknownMethod extracts the expected parameters from the error's ParameterList.
func ParamsErrUnknownMethod(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfUnknownSuffix calls ErrUnknownSuffix.Errorf with the supplied arguments.
func ErrorfUnknownSuffix(ctx *context.T, format string) error {
	return ErrUnknownSuffix.Errorf(ctx, format)
}

// MessageUnknownSuffix calls ErrUnknownSuffix.Message with the supplied arguments.
func MessageUnknownSuffix(ctx *context.T, message string) error {
	return ErrUnknownSuffix.Message(ctx, message)
}

// ParamsErrUnknownSuffix extracts the expected parameters from the error's ParameterList.
func ParamsErrUnknownSuffix(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfNoExistOrNoAccess calls ErrNoExistOrNoAccess.Errorf with the supplied arguments.
func ErrorfNoExistOrNoAccess(ctx *context.T, format string) error {
	return ErrNoExistOrNoAccess.Errorf(ctx, format)
}

// MessageNoExistOrNoAccess calls ErrNoExistOrNoAccess.Message with the supplied arguments.
func MessageNoExistOrNoAccess(ctx *context.T, message string) error {
	return ErrNoExistOrNoAccess.Message(ctx, message)
}

// ParamsErrNoExistOrNoAccess extracts the expected parameters from the error's ParameterList.
func ParamsErrNoExistOrNoAccess(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfNoServers calls ErrNoServers.Errorf with the supplied arguments.
func ErrorfNoServers(ctx *context.T, format string) error {
	return ErrNoServers.Errorf(ctx, format)
}

// MessageNoServers calls ErrNoServers.Message with the supplied arguments.
func MessageNoServers(ctx *context.T, message string) error {
	return ErrNoServers.Message(ctx, message)
}

// ParamsErrNoServers extracts the expected parameters from the error's ParameterList.
func ParamsErrNoServers(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfNoAccess calls ErrNoAccess.Errorf with the supplied arguments.
func ErrorfNoAccess(ctx *context.T, format string) error {
	return ErrNoAccess.Errorf(ctx, format)
}

// MessageNoAccess calls ErrNoAccess.Message with the supplied arguments.
func MessageNoAccess(ctx *context.T, message string) error {
	return ErrNoAccess.Message(ctx, message)
}

// ParamsErrNoAccess extracts the expected parameters from the error's ParameterList.
func ParamsErrNoAccess(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfNotTrusted calls ErrNotTrusted.Errorf with the supplied arguments.
func ErrorfNotTrusted(ctx *context.T, format string) error {
	return ErrNotTrusted.Errorf(ctx, format)
}

// MessageNotTrusted calls ErrNotTrusted.Message with the supplied arguments.
func MessageNotTrusted(ctx *context.T, message string) error {
	return ErrNotTrusted.Message(ctx, message)
}

// ParamsErrNotTrusted extracts the expected parameters from the error's ParameterList.
func ParamsErrNotTrusted(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfAborted calls ErrAborted.Errorf with the supplied arguments.
func ErrorfAborted(ctx *context.T, format string) error {
	return ErrAborted.Errorf(ctx, format)
}

// MessageAborted calls ErrAborted.Message with the supplied arguments.
func MessageAborted(ctx *context.T, message string) error {
	return ErrAborted.Message(ctx, message)
}

// ParamsErrAborted extracts the expected parameters from the error's ParameterList.
func ParamsErrAborted(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfBadProtocol calls ErrBadProtocol.Errorf with the supplied arguments.
func ErrorfBadProtocol(ctx *context.T, format string) error {
	return ErrBadProtocol.Errorf(ctx, format)
}

// MessageBadProtocol calls ErrBadProtocol.Message with the supplied arguments.
func MessageBadProtocol(ctx *context.T, message string) error {
	return ErrBadProtocol.Message(ctx, message)
}

// ParamsErrBadProtocol extracts the expected parameters from the error's ParameterList.
func ParamsErrBadProtocol(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfCanceled calls ErrCanceled.Errorf with the supplied arguments.
func ErrorfCanceled(ctx *context.T, format string) error {
	return ErrCanceled.Errorf(ctx, format)
}

// MessageCanceled calls ErrCanceled.Message with the supplied arguments.
func MessageCanceled(ctx *context.T, message string) error {
	return ErrCanceled.Message(ctx, message)
}

// ParamsErrCanceled extracts the expected parameters from the error's ParameterList.
func ParamsErrCanceled(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

// ErrorfTimeout calls ErrTimeout.Errorf with the supplied arguments.
func ErrorfTimeout(ctx *context.T, format string) error {
	return ErrTimeout.Errorf(ctx, format)
}

// MessageTimeout calls ErrTimeout.Message with the supplied arguments.
func MessageTimeout(ctx *context.T, message string) error {
	return ErrTimeout.Message(ctx, message)
}

// ParamsErrTimeout extracts the expected parameters from the error's ParameterList.
func ParamsErrTimeout(argumentError error) (verrorComponent string, verrorOperation string, returnErr error) {
	params := Params(argumentError)
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

	return struct{}{}
}
