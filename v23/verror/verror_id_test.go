// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package verror_test

import (
	"testing"

	"v.io/v23/flow"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security"
)

func TestPackageErrorIDs(t *testing.T) {
	for i, tc := range []struct {
		err error
		id  string
	}{
		{verror.ErrInternal.Errorf(nil, "x"), "v.io/v23/verror.Internal"},
		{flow.ErrAuth.Errorf(nil, "x"), "v.io/v23/flow.Auth"},
		{security.ErrBadPassphrase.Errorf(nil, "x"), "v.io/x/ref/lib/security.errBadPassphrase"},
	} {
		if got, want := verror.ErrorID(tc.err), verror.ID(tc.id); got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
	}
}
