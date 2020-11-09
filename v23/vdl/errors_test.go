// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl_test

import (
	"os"
	"runtime"
	"testing"

	"v.io/v23/context"
	"v.io/v23/vdl/vdltest"
	"v.io/v23/verror"
)

func TestErrorParams(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	ctx = verror.WithComponentName(ctx, "my-component")

	var (
		returnErr            error
		component, operation string
		oneI                 int64
		twoA                 string
		twoErr               error
	)

	assert := func() {
		_, _, line, _ := runtime.Caller(1)
		if returnErr != nil {
			t.Fatalf("line %v: unexpected error: %v", line, returnErr)
		}
		if got, want := component, "my-component"; got != want {
			t.Fatalf("line %v: got %v, want: %v", line, got, want)
		}
		if got, want := operation, ""; got != want {
			t.Fatalf("line %v: got %v, want: %v", line, got, want)
		}
	}

	expectError := func(msg string) {
		_, _, line, _ := runtime.Caller(1)
		if returnErr == nil || returnErr.Error() != msg {
			t.Errorf("line %v: expected a specific error, got %v", line, returnErr)
		}
	}

	err := vdltest.ErrorfNone(ctx, "an error")
	component, operation, returnErr = vdltest.ParamsErrNone(err)
	assert()
	err = vdltest.ErrorfOne(ctx, "an error", 33)
	component, operation, oneI, returnErr = vdltest.ParamsErrOne(err)
	assert()
	err = vdltest.ErrorfTwo(ctx, "an error", "oops", os.ErrNotExist)
	component, operation, twoA, twoErr, returnErr = vdltest.ParamsErrTwo(err)
	assert()

	if got, want := oneI, int64(33); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := twoA, "oops"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := twoErr.Error(), os.ErrNotExist.Error(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	err = vdltest.ErrorfOne(ctx, "an error", 66)
	err = verror.WithSubErrors(err, verror.SubErr{Name: "test", Err: os.ErrExist})
	component, operation, oneI, returnErr = vdltest.ParamsErrOne(err)
	assert()
	if got, want := oneI, int64(66); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	wrongErr := vdltest.ErrorfNone(ctx, "an error")
	_, _, _, _, returnErr = vdltest.ParamsErrTwo(wrongErr)
	expectError("too few parameters: have 2")

	wrongErr = verror.ErrUnknown.Errorf(ctx, "some random types: %v", "a string")
	_, _, _, _, returnErr = vdltest.ParamsErrTwo(wrongErr)
	expectError("too few parameters: have 3")

	wrongErr = verror.ErrUnknown.Errorf(ctx, "some random types: %v", os.ErrExist)
	_, _, _, returnErr = vdltest.ParamsErrOne(wrongErr)
	expectError("parameter list contains the wrong type for return value i, has *errors.errorString and not int64")

	wrongErr = verror.ErrUnknown.Errorf(ctx, "some random types: %v %v", "oh", 33.33)
	_, _, _, _, _, returnErr = vdltest.ParamsErrThree(wrongErr)
	expectError("parameter list contains the wrong type for return value b, has float64 and not int64")

	_, _, returnErr = vdltest.ParamsErrNone(os.ErrClosed)
	expectError("no parameters found in: *errors.errorString: file already closed")

	verr := verror.E{ID: "x:"}
	_, _, returnErr = vdltest.ParamsErrNone(verr)
	expectError("too few parameters: have 0")
	verr.ParamList = []interface{}{"a"}
	_, _, returnErr = vdltest.ParamsErrNone(verr)
	expectError("too few parameters: have 1")
	verr.ParamList = []interface{}{"a", 2}
	_, _, returnErr = vdltest.ParamsErrNone(verr)
	expectError("ParamList[1]: operation name is not a string: int")
	verr.ParamList = []interface{}{33.0, "a"}
	_, _, returnErr = vdltest.ParamsErrNone(verr)
	expectError("ParamList[0]: component name is not a string: float64")
}
