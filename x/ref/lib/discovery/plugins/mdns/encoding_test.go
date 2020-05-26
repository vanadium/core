// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mdns

import (
	"crypto/rand"
	"encoding/base64"
	"reflect"
	"sort"
	"testing"

	"v.io/v23/discovery"
)

func TestEncodeAdID(t *testing.T) {
	for i := 0; i < 10; i++ {
		var id discovery.AdId
		_, err := rand.Read(id[:])
		if err != nil {
			panic(err)
		}

		encoded := encodeAdID(&id)

		var decoded discovery.AdId
		if err := decodeAdID(encoded, &decoded); err != nil {
			t.Errorf("decode id failed: %v", err)
			continue
		}
		if id != decoded {
			t.Errorf("decoded to %v, but want %v", decoded, id)
		}
	}
}

func TestEncodeLargeTxt(t *testing.T) {
	tests := [][]string{
		{randTxt(maxTxtRecordLen / 2)},
		{randTxt(maxTxtRecordLen / 2), randTxt(maxTxtRecordLen / 3)},
		{randTxt(maxTxtRecordLen * 2)},
		{randTxt(maxTxtRecordLen * 2), randTxt(maxTxtRecordLen * 3)},
		{randTxt(maxTxtRecordLen / 2), randTxt(maxTxtRecordLen * 3), randTxt(maxTxtRecordLen * 2), randTxt(maxTxtRecordLen / 3)},
	}

	for i, test := range tests {
		var splitted []string
		for i, txt := range test {
			xseq := i*3 + 1 // To test non-sequential index.
			txts, nxseq := maybeSplitTxtRecord(txt, xseq)
			if len(txts) > 1 {
				xseq++
			}
			if nxseq != xseq {
				t.Errorf("[%d]: got xseq %d; but wanted %d", i, nxseq, xseq)
			}
			splitted = append(splitted, txts...)
		}

		for _, v := range splitted {
			if len(v) > maxTxtRecordLen {
				t.Errorf("[%d]: too large encoded txt %d - %v", i, len(v), v)
			}
		}

		txts, err := maybeJoinTxtRecords(splitted)
		if err != nil {
			t.Errorf("[%d]: decodeLargeTxt failed: %v", i, err)
			continue
		}

		sort.Strings(txts)
		sort.Strings(test)
		if !reflect.DeepEqual(txts, test) {
			t.Errorf("[%d]: decoded to %v, but want %v", i, txts, test)
		}
	}
}

func randTxt(n int) string {
	b := make([]byte, (n*3+3)/4)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.RawStdEncoding.EncodeToString(b)[:n]
}
