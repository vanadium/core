// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fake

import (
	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
)

// common implements some common functionality of the other fakes.
type common struct{}

func (*common) SetPermissions(*context.T, access.Permissions, string) error { return nil }

func (*common) GetPermissions(*context.T) (access.Permissions, string, error) {
	return nil, "", nil
}
func (*common) Id() wire.Id                  { return wire.Id{Blessing: "a-blessing", Name: "a-name"} }
func (*common) Destroy(ctx *context.T) error { return nil }
