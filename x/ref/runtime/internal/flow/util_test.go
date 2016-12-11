// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flow

import (
	"errors"
	"testing"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/verror"
)

func TestMaybeWrapError(t *testing.T) {
	ctx, _ := context.RootContext()
	tests := []struct {
		err  error
		wrap bool
	}{
		{nil, true},
		{errors.New("wrap this error"), true},
		{verror.NewErrUnknown(ctx), false},
		{flow.NewErrAuth(ctx, nil), false},
	}
	for _, test := range tests {
		werr := MaybeWrapError(flow.ErrAuth, ctx, test.err)
		// If the returned error is not equal to the original error it was wrapped.
		msg := ""
		if test.err != nil {
			msg = test.err.Error()
		}
		if wasWrapped := werr.Error() != msg; wasWrapped != test.wrap {
			if test.wrap {
				t.Errorf("wanted %v to be wrapped", test.err)
			} else {
				t.Errorf("did not want %v to be wrapped", test.err)
			}
		}
	}
}
