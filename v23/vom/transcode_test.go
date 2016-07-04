// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom_test

import (
	"bytes"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom"
	"v.io/v23/vom/vomtest"
)

func TestTranscodeDecoderToEncoder(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		var buf bytes.Buffer
		enc := vom.NewVersionedEncoder(test.Version, &buf)
		dec := vom.NewDecoder(bytes.NewReader(test.Bytes()))
		if err := vdl.Transcode(enc.Encoder(), dec.Decoder()); err != nil {
			t.Errorf("%s: Transcode failed: %v", test.Name(), err)
			continue
		}
		if got, want := buf.Bytes(), test.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("%s\nGOT  %x\nWANT %x", test.Name(), got, want)
		}
	}
}

// TODO(bprosnitz) This is probably not the right place for this test.
func TestTranscodeVDLValueToEncoder(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		var buf bytes.Buffer
		enc := vom.NewEncoder(&buf)
		if err := vdl.Write(enc.Encoder(), test.Value.Interface()); err != nil {
			t.Errorf("%s: vdl.Write failed: %v", test.Name(), err)
			continue
		}
		if got, want := buf.Bytes(), test.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("%s\nGOT  %x\nWANT %x", test.Name(), got, want)
		}
	}
}
