// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"v.io/v23/discovery"
)

func TestHashAd(t *testing.T) {
	a1 := AdInfo{
		Ad: discovery.Advertisement{
			Id:            discovery.AdId{1, 2, 3},
			InterfaceName: "v.io/x",
			Addresses: []string{
				"/@6@wsh@foo.com:1234@@/x",
			},
			Attributes: discovery.Attributes{
				"k1": "v1",
				"k2": "v2",
			},
		}}

	// Shouldn't be changed by hashing.
	a2 := a1
	hashAd(&a2)
	a2.Hash = AdHash{}
	if !reflect.DeepEqual(a1, a2) {
		t.Errorf("shouldn't be changed by hash: %v, %v", a1, a2)
	}

	// Should have the same hash for the same advertisements.
	hashAd(&a1)
	hashAd(&a2)
	if a1.Hash != a2.Hash {
		t.Errorf("expected same hash, but got different: %v, %v", a1.Hash, a2.Hash)
	}

	// Should be idempotent.
	hashAd(&a2)
	if a1.Hash != a2.Hash {
		t.Errorf("expected same hash, but got different: %v, %v", a1.Hash, a2.Hash)
	}

	a2.Ad.Id = discovery.AdId{4, 5, 6}
	hashAd(&a2)
	if a1.Hash == a2.Hash {
		t.Errorf("expected different hashes, but got same: %v, %v", a1.Hash, a2.Hash)
	}

	// Should distinguish between {"", "x"} and {"x", ""}.
	a2 = a1
	a2.Ad.Attributes = nil
	a2.Ad.Attachments = make(discovery.Attachments)
	for k, v := range a1.Ad.Attributes {
		a2.Ad.Attachments[k] = []byte(v)
	}
	hashAd(&a2)
	if a1.Hash == a2.Hash {
		t.Errorf("expected different hashes, but got same: %v, %v", a1.Hash, a2.Hash)
	}

	// Shouldn't distinguish map order.
	a2 = a1
	a2.Ad.Attributes = make(discovery.Attributes)
	var keys []string
	for k := range a1.Ad.Attributes {
		keys = append(keys, k)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		a2.Ad.Attributes[keys[i]] = a1.Ad.Attributes[keys[i]]
	}
	hashAd(&a2)
	if a1.Hash != a2.Hash {
		t.Errorf("expected same hash, but got different: %v, %v", a1.Hash, a2.Hash)
	}

	// Shouldn't distinguish between nil and empty.
	a2 = a1
	a2.Ad.Attachments = make(discovery.Attachments)
	hashAd(&a2)
	if a1.Hash != a2.Hash {
		t.Errorf("expected same hash, but got different: %v, %v", a1.Hash, a2.Hash)
	}
}

func TestHashAdCoverage(t *testing.T) {
	rand := rand.New(rand.NewSource(0))
	gen := func(v reflect.Value) {
		for {
			r, ok := quick.Value(v.Type(), rand)
			if !ok {
				t.Fatalf("failed to populate value for %v", v)
			}
			switch v.Kind() {
			case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
				if r.Len() == 0 {
					continue
				}
			}
			if reflect.DeepEqual(v, r) {
				continue
			}
			v.Set(r)
			return
		}
	}

	// Ensure that every single field of advertisement is hashed.
	ad := AdInfo{}
	hashAd(&ad)

	for ty, i := reflect.TypeOf(ad.Ad), 0; i < ty.NumField(); i++ {
		oldAd := ad

		fieldName := reflect.TypeOf(ad.Ad).Field(i).Name

		gen(reflect.ValueOf(&ad.Ad).Elem().FieldByName(fieldName))
		hashAd(&ad)

		if oldAd.Hash == ad.Hash {
			t.Errorf("Ad.%s: expected different hashes, but got same: %v, %v", fieldName, oldAd.Hash, ad.Hash)
		}
	}

	for ty, i := reflect.TypeOf(ad), 0; i < ty.NumField(); i++ {
		oldAd := ad

		fieldName := reflect.TypeOf(ad).Field(i).Name

		switch fieldName {
		case "Ad", "Hash", "TimestampNs", "DirAddrs", "Status", "Lost":
			continue
		}

		gen(reflect.ValueOf(&ad).Elem().FieldByName(fieldName))
		hashAd(&ad)

		if oldAd.Hash == ad.Hash {
			t.Errorf("AdInfo.%s: expected different hashes, but got same: %v, %v", fieldName, oldAd.Hash, ad.Hash)
		}
	}
}
