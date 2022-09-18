// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package vom

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestBinaryEncodeDecode(t *testing.T) { //nolint:gocyclo
	tests := []struct {
		v   interface{}
		hex string
	}{
		{false, "00"},
		{true, "01"},
		{byte(0x80), "80"},
		{byte(0xbf), "bf"},
		{byte(0xc0), "c0"},
		{byte(0xdf), "df"},
		{byte(0xe0), "e0"},
		{byte(0xef), "ef"},
		{uint64(0), "00"},
		{uint64(1), "01"},
		{uint64(2), "02"},
		{uint64(127), "7f"},
		{uint64(128), "ff80"},
		{uint64(255), "ffff"},
		{uint64(256), "fe0100"},
		{uint64(257), "fe0101"},
		{uint64(0xffff), "feffff"},
		{uint64(0xffffff), "fdffffff"},
		{uint64(0xffffffff), "fcffffffff"},
		{uint64(0xffffffffff), "fbffffffffff"},
		{uint64(0xffffffffffff), "faffffffffffff"},
		{uint64(0xffffffffffffff), "f9ffffffffffffff"},
		{uint64(0xffffffffffffffff), "f8ffffffffffffffff"},
		{int64(0), "00"},
		{int64(1), "02"},
		{int64(2), "04"},
		{int64(63), "7e"},
		{int64(64), "ff80"},
		{int64(65), "ff82"},
		{int64(127), "fffe"},
		{int64(128), "fe0100"},
		{int64(129), "fe0102"},
		{int64(math.MaxInt16), "fefffe"},
		{int64(math.MaxInt32), "fcfffffffe"},
		{int64(math.MaxInt64), "f8fffffffffffffffe"},
		{int64(-1), "01"},
		{int64(-2), "03"},
		{int64(-64), "7f"},
		{int64(-65), "ff81"},
		{int64(-66), "ff83"},
		{int64(-128), "ffff"},
		{int64(-129), "fe0101"},
		{int64(-130), "fe0103"},
		{int64(math.MinInt16), "feffff"},
		{int64(math.MinInt32), "fcffffffff"},
		{int64(math.MinInt64), "f8ffffffffffffffff"},
		{float64(0), "00"},
		{float64(1), "fef03f"},
		{float64(17), "fe3140"},
		{float64(18), "fe3240"},
		{"", "00"},
		{"abc", "03616263"},
		{"defghi", "06646566676869"},
	}
	for _, test := range tests {
		// Test encode
		encbuf := newEncbuf()
		var buf []byte
		switch val := test.v.(type) {
		case byte:
			binaryEncodeControl(encbuf, val)
		case bool:
			binaryEncodeBool(encbuf, val)
		case uint64:
			binaryEncodeUint(encbuf, val)
			buf = make([]byte, maxEncodedUintBytes)
			buf = buf[binaryEncodeUintEnd(buf, val):]
		case int64:
			binaryEncodeInt(encbuf, val)
			buf = make([]byte, maxEncodedUintBytes)
			buf = buf[binaryEncodeIntEnd(buf, val):]
		case float64:
			binaryEncodeFloat(encbuf, val)
		case string:
			binaryEncodeString(encbuf, val)
		}
		if got, want := fmt.Sprintf("%x", encbuf.Bytes()), test.hex; got != want {
			t.Errorf("binary encode %T(%v): GOT 0x%v WANT 0x%v", test.v, test.v, got, want)
		}
		if buf != nil {
			if got, want := fmt.Sprintf("%x", buf), test.hex; got != want {
				t.Errorf("binary encode end %T(%v): GOT 0x%v WANT 0x%v", test.v, test.v, got, want)
			}
		}
		// Test decode
		var bin string
		if _, err := fmt.Sscanf(test.hex, "%x", &bin); err != nil {
			t.Errorf("couldn't scan 0x%v as hex: %v", test.hex, err)
			continue
		}
		// TODO(toddw): Add peek tests.
		decbuf := newDecbuf(strings.NewReader(bin))
		decbufSkip := newDecbuf(strings.NewReader(bin))
		var v interface{}
		var err, errSkip error
		switch tv := test.v.(type) {
		case byte:
			v = test.v
			var match bool
			if match, err = binaryDecodeControlOnly(decbuf, tv); !match {
				err = fmt.Errorf("non control byte")
			}
			errSkip = decbufSkip.Skip(1)
		case bool:
			v, err = binaryDecodeBool(decbuf)
			errSkip = binarySkipUint(decbufSkip)
		case uint64:
			v, err = binaryDecodeUint(decbuf)
			errSkip = binarySkipUint(decbufSkip)
		case int64:
			v, err = binaryDecodeInt(decbuf)
			errSkip = binarySkipUint(decbufSkip)
		case float64:
			v, err = binaryDecodeFloat(decbuf)
			errSkip = binarySkipUint(decbufSkip)
		case string:
			v, err = binaryDecodeString(decbuf)
			errSkip = binarySkipString(decbufSkip)
		}
		// Check decode results
		if err != nil {
			t.Errorf("binary decode %T(0x%v): %v", test.v, test.hex, err)
			continue
		}
		if !reflect.DeepEqual(v, test.v) {
			t.Errorf("binary decode %T(0x%v): got %v, want %v", test.v, test.hex, v, test.v)
			continue
		}
		if err := decbuf.Fill(1); err == nil {
			t.Errorf("binary decode %T(0x%v): leftover bytes", test.v, test.hex)
			continue
		}
		// Check skip results
		if errSkip != nil {
			t.Errorf("binary skip %T(0x%v): %v", test.v, test.hex, errSkip)
			continue
		}
		if err := decbufSkip.Fill(1); err == nil {
			t.Errorf("binary skip %T(0x%v): leftover bytes", test.v, test.hex)
			continue
		}
	}
}
