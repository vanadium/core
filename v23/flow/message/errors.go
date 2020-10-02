// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package message

import (
	"errors"

	"v.io/v23/context"
	"v.io/v23/verror"
)

var (
	ErrInvalidMsg         = verror.NewID("InvalidMsg")
	ErrInvalidSetupOption = verror.NewID("InvalidSetupOption")
	ErrUnknownMsg         = verror.NewID("UnknownMsg")
	ErrMissingBlessings   = verror.NewID("MissingBlessings")
)

// NewErrInvalidMsg creates an instance ErrInvalidMsg with the specified parameters which can be
// access by ParseErrInvalidMessage.
func NewErrInvalidMsg(ctx *context.T, typ byte, size uint64, field uint64, err error) error {
	return ErrInvalidMsg.Errorf(ctx, "message of type %v and size %v failed decoding at filed %v:%v", typ, size, field, err)
}

// ParseErrInvalidMessage cracks open the parameter list created by NewErrInvalidMsg with
// appropriate checking. ok will be false if all of the error parameters are not located.
func ParseErrInvalidMessage(err error) (typ byte, size uint64, field uint64, ok bool) {
	if !errors.Is(err, ErrInvalidMsg) {
		return
	}
	verr, ok := err.(verror.E)
	if !ok {
		if v, _ := err.(*verror.E); v != nil {
			verr = *v
		}
	}
	next := 2
	candidate := func() interface{} {
		if next >= len(verr.ParamList) {
			return nil
		}
		c := verr.ParamList[next]
		next++
		return c
	}
	typ, ok = candidate().(byte)
	if !ok {
		return
	}
	size, ok = candidate().(uint64)
	if !ok {
		return
	}
	field, ok = candidate().(uint64)
	if !ok {
		return
	}
	ok = true
	return
}
