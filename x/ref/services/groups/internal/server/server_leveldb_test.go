// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build leveldb

package server_test

import (
	"testing"
)

func TestCreateLevelDBStore(t *testing.T) {
	testCreateHelper(t, leveldbstore)
}

func TestDeleteLevelDBStore(t *testing.T) {
	testDeleteHelper(t, leveldbstore)
}

func TestPermsLevelDBStore(t *testing.T) {
	testPermsHelper(t, leveldbstore)
}

func TestAddLevelDBStore(t *testing.T) {
	testAddHelper(t, leveldbstore)
}

func TestRemoveLevelDBStore(t *testing.T) {
	testRemoveHelper(t, leveldbstore)
}
