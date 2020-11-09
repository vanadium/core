// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gotagtest_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"v.io/x/ref/cmd/vdl/gotagtest"
)

func TestTags(t *testing.T) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.Encode(gotagtest.TestStructA{"val", 64})
	if got, want := buf.String(), `{"ja":"val","B":64}`+"\n"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	buf.Reset()
	enc.Encode(gotagtest.TestStructB{64, ""})
	if got, want := buf.String(), `{"A":64}`+"\n"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
