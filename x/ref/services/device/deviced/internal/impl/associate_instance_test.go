// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestSystemNameState(t *testing.T) {
	dir, err := ioutil.TempDir("", "instance")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	expected := "vanadium-user"
	if err := saveSystemNameForInstance(dir, expected); err != nil {
		t.Fatalf("saveSystemNameForInstance(%v, %v) failed: %v", dir, expected, err)
	}

	got, err := readSystemNameForInstance(dir)
	if err != nil {
		t.Fatalf("readSystemNameForInstance(%v) failed: ", err)
	}
	if got != expected {
		t.Fatalf("got %v, expected %v", got, expected)
	}
}
