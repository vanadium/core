// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build fuzzdump

package vom_test

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"v.io/v23/vom/vomtest"
)

// Add the vom tests as binary examples for the fuzzer.
// This is more of a script than a test, but is written as a test
// to access binFromHexPat.
func TestFuzzDump(t *testing.T) {
	if err := os.MkdirAll("fuzz-workdir/corpus", 0755); err != nil {
		t.Fatal(err)
	}

	seen := make(map[[sha1.Size]byte]bool)

	for _, test := range vomtest.AllPass() {
		data := test.Bytes()
		hash := sha1.Sum(data)
		if seen[hash] {
			continue
		}
		seen[hash] = true

		name := fmt.Sprintf("fuzz-workdir/corpus/%x", hash)
		if err := ioutil.WriteFile(name, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
