// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"v.io/v23/security/access"
	"v.io/x/ref/services/syncbase/server/util"
)

var (
	_ util.Permser = (*ServiceData)(nil)
	_ util.Permser = (*DatabaseData)(nil)
)

func (data *ServiceData) GetPerms() access.Permissions {
	return data.Perms
}

func (data *DatabaseData) GetPerms() access.Permissions {
	return data.Perms
}
