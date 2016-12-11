// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !leveldb

package leveldb

import (
	"fmt"

	"v.io/x/ref/services/groups/internal/store"
)

func Open(path string) (store.Store, error) {
	return nil, fmt.Errorf("use the 'leveldb' build tag to build groupsd with leveldb storage engine")
}
