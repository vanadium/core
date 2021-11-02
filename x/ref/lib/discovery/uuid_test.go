// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery_test

import (
	"testing"

	"github.com/google/uuid"
	"v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/discovery/testdata"
)

func TestServiceUUID(t *testing.T) {
	var tmp uuid.UUID
	for _, test := range testdata.ServiceUuidTest {
		copy(tmp[:], discovery.NewServiceUUID(test.In))
		if got := tmp.String(); got != test.Want {
			t.Errorf("ServiceUUID for %q mismatch; got %q, want %q", test.In, got, test.Want)
		}
	}
}

func TestAttributeUUID(t *testing.T) {
	var tmp uuid.UUID
	for _, test := range testdata.AttributeUuidTest {
		copy(tmp[:], discovery.NewAttributeUUID(test.In))
		if got := tmp.String(); got != test.Want {
			t.Errorf("AttributeUUID for %q mismatch; got %q, want %q", test.In, got, test.Want)
		}
	}
}
