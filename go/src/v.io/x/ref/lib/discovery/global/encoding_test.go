// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package global

import (
	"reflect"
	"testing"
	"time"

	"v.io/v23/discovery"
)

func TestAdSuffix(t *testing.T) {
	testCases := []testCase{
		{
			ad: &discovery.Advertisement{
				Id:            discovery.AdId{1, 2, 3},
				InterfaceName: "foo",
			},
			timestampNs: time.Now().UnixNano(),
		},
		{
			ad: &discovery.Advertisement{
				Attributes:    discovery.Attributes{"k": "v"},
				InterfaceName: "foo",
			},
			timestampNs: time.Now().UnixNano(),
		},
		{
			ad: &discovery.Advertisement{
				Id:            discovery.AdId{1, 2, 3},
				Attributes:    discovery.Attributes{"k": "v"},
				InterfaceName: "foo",
			},
			timestampNs: time.Now().UnixNano(),
		},
	}
	for i, want := range testCases {
		encAd, err := encodeAdToSuffix(want.ad, want.timestampNs)
		if err != nil {
			t.Error(err)
		}
		var got testCase
		got.ad, got.timestampNs, err = decodeAdFromSuffix(encAd)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("#%d: got %#v, want %#v", i, got, want)
		}
	}
}

type testCase struct {
	ad          *discovery.Advertisement
	timestampNs int64
}
