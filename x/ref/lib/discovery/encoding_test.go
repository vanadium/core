// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery_test

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/discovery/testdata"
	"v.io/x/ref/lib/security/bcrypter"
)

func TestEncodingBuffer(t *testing.T) {
	rand := rand.New(rand.NewSource(0))
	randBytes := func() []byte {
		p := make([]byte, rand.Intn(128))
		rand.Read(p)
		return p
	}

	var data []interface{}

	enc := idiscovery.NewEncodingBuffer(nil)
	for i := 0; i < 10; i++ {
		switch rand.Intn(3) {
		case 0:
			x := rand.Int()
			enc.WriteInt(x)
			data = append(data, x)
		case 1:
			p := randBytes()
			enc.WriteBytes(p)
			data = append(data, p)
		case 2:
			s := string(randBytes())
			enc.WriteString(s)
			data = append(data, s)
		}
	}

	dec := idiscovery.NewEncodingBuffer(enc.Bytes())
	for _, d := range data {
		switch d := d.(type) {
		case int:
			x, err := dec.ReadInt()
			if err != nil {
				t.Error(err)
			} else if x != d {
				t.Errorf("decoded to %v, but want %v", x, d)
			}
		case []byte:
			p, err := dec.ReadBytes()
			if err != nil {
				t.Error(err)
			} else if !bytes.Equal(p, d) {
				t.Errorf("decoded to %v, but want %v", p, d)
			}
		case string:
			s, err := dec.ReadString()
			if err != nil {
				t.Error(err)
			} else if s != d {
				t.Errorf("decoded to %v, but want %v", s, d)
			}
		}
	}
}

func TestPackAddresses(t *testing.T) {
	for _, test := range testdata.PackAddressTestData {
		pack := idiscovery.PackAddresses(test.In)
		if !reflect.DeepEqual(pack, test.Packed) {
			t.Errorf("packed to: %v (%d bytes), but wanted: %v (%d bytes)", pack, len(pack), test.Packed, len(test.Packed))
		}
		unpack, err := idiscovery.UnpackAddresses(test.Packed)
		if err != nil {
			t.Errorf("unpack error: %v", err)
			continue
		}
		if !reflect.DeepEqual(unpack, test.In) {
			t.Errorf("unpacked to %v, but want %v", unpack, test.In)
		}
	}
}

func TestPackEncryptionKeys(t *testing.T) {
	for _, test := range testdata.PackEncryptionKeysTestData {
		pack := idiscovery.PackEncryptionKeys(test.Algo, test.Keys)

		if !reflect.DeepEqual(pack, test.Packed) {
			t.Errorf("packed to: %v, but wanted: %v", pack, test.Packed)
		}

		algo, keys, err := idiscovery.UnpackEncryptionKeys(test.Packed)
		if err != nil {
			t.Errorf("unpack error: %v", err)
			continue
		}
		if algo != test.Algo || !reflect.DeepEqual(keys, test.Keys) {
			t.Errorf("unpacked to (%v, %v), but want (%v, %v)", algo, keys, test.Algo, test.Keys)
		}
	}
}

func TestEncodeWireCiphertext(t *testing.T) {
	rand := rand.New(rand.NewSource(0))
	for i := 0; i < 1; i++ {
		v, ok := quick.Value(reflect.TypeOf(bcrypter.WireCiphertext{}), rand)
		if !ok {
			t.Fatal("failed to populate value")
		}
		wctext := v.Interface().(bcrypter.WireCiphertext)
		// Make reflect.DeepEqual happy in comparing nil and empty.
		if wctext.Bytes == nil {
			wctext.Bytes = make(map[string][]byte)
		}

		encoded := idiscovery.EncodeWireCiphertext(&wctext)
		decoded, err := idiscovery.DecodeWireCiphertext(encoded)
		if err != nil {
			t.Errorf("decoded error: %v", err)
			continue
		}
		if !reflect.DeepEqual(decoded, &wctext) {
			t.Errorf("decoded to %v, but want %v", *decoded, wctext)
		}
	}
}
