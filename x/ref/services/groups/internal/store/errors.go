// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import "v.io/v23/verror"

var (
	// ErrKeyExists means the given key already exists in the store.
	ErrKeyExists = verror.NewID("KeyExists")
	// ErrUnknownKey means the given key does not exist in the store.
	ErrUnknownKey = verror.NewID("UnknownKey")
)
