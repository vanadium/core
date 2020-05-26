// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ble

import (
	"encoding/hex"
	"math/rand"
	"reflect"
	"testing"

	"v.io/v23/discovery"
	idiscovery "v.io/x/ref/lib/discovery"
)

func TestEncode(t *testing.T) { //nolint:gocyclo
	rand := rand.New(rand.NewSource(0))
	randBytes := func(n int) []byte {
		p := make([]byte, rand.Intn(n))
		rand.Read(p)
		return p
	}
	randString := func(n int) string {
		return string(randBytes(n))
	}

	for i := 0; i < 10; i++ {
		encodedAdinfos := make(map[discovery.AdId][]byte)
		expectedAdinfos := make(map[discovery.AdId]idiscovery.AdInfo)

		for j, n := 0, rand.Intn(maxNumPackedServices); j < n; j++ {
			adinfo := idiscovery.AdInfo{}

			// Populate adinfo manually to test large payloads. testing.quick.Value()
			// fills only with small values.
			copy(adinfo.Ad.Id[:], randBytes(32))
			adinfo.Ad.InterfaceName = randString(128)
			adinfo.Ad.Addresses = make([]string, rand.Intn(3)+1)
			for i := range adinfo.Ad.Addresses {
				adinfo.Ad.Addresses[i] = randString(128)
			}

			if n := rand.Intn(5); n > 0 {
				adinfo.Ad.Attributes = make(discovery.Attributes, n)
				for i := 0; i < n; i++ {
					adinfo.Ad.Attributes[randString(16)] = randString(256)
				}
			}
			if n := rand.Intn(5); n > 0 {
				adinfo.Ad.Attachments = make(discovery.Attachments, n)
				for i := 0; i < n; i++ {
					adinfo.Ad.Attachments[randString(16)] = randBytes(256)
				}
			}

			adinfo.EncryptionAlgorithm = idiscovery.EncryptionAlgorithm(rand.Intn(3))
			if adinfo.EncryptionAlgorithm != idiscovery.NoEncryption {
				adinfo.EncryptionKeys = make([]idiscovery.EncryptionKey, rand.Intn(3)+1)
				for i := range adinfo.EncryptionKeys {
					adinfo.EncryptionKeys[i] = randBytes(128)
				}
			}

			copy(adinfo.Hash[:], randBytes(16))
			adinfo.TimestampNs = rand.Int63()

			adinfo.DirAddrs = make([]string, rand.Intn(3)+1)
			for i := range adinfo.DirAddrs {
				adinfo.DirAddrs[i] = randString(128)
			}

			encoded, err := encodeAdInfo(&adinfo)
			if err != nil {
				t.Errorf("encode failed: %v", err)
				continue
			}
			encodedAdinfos[adinfo.Ad.Id] = encoded

			if len(adinfo.Ad.Attachments) > 0 {
				adinfo.Status = idiscovery.AdPartiallyReady
			} else {
				adinfo.DirAddrs = nil
				adinfo.Status = idiscovery.AdReady
			}
			adinfo.Ad.Attachments = nil
			expectedAdinfos[adinfo.Ad.Id] = adinfo
		}

		cs := packToCharacteristics(encodedAdinfos)
		unpacked, err := unpackFromCharacteristics(cs)
		if err != nil {
			t.Errorf("unpack failed: %v", err)
			continue
		}

		for _, encoded := range unpacked {
			adinfo, err := decodeAdInfo(encoded)
			if err != nil {
				t.Errorf("decode failed: %v", err)
				continue
			}

			if !reflect.DeepEqual(*adinfo, expectedAdinfos[adinfo.Ad.Id]) {
				t.Errorf("decoded to %#v, but want %#v", *adinfo, expectedAdinfos[adinfo.Ad.Id])
			}
		}
	}
}

func TestDecodeInvalid(t *testing.T) {
	tests := []map[string]string{
		{"1234": "00"}, // Invalid uuid.
		{"31ca10d5-0195-54fa-9344-25fcd7072f00": "00"}, // Invalid characteristic uuid.
		{"31ca10d5-0195-54fa-9344-25fcd7072e10": "00"}, // Invalid characteristic uuid sequence.
		{ //  Invalid characteristic split.
			"31ca10d5-0195-54fa-9344-25fcd7072e00": "08eff4d54d979febed94a1b551b0316733762e696f2f782f7265662f73657276696365732f73796e63626173652f7365727665722f696e74657266616365732f53796e63fd01d4ce4d4ec33010c57171205c3b76c633b3321b2e8158f8b30db471459cf4442c90e00270390a4854e204416ff7167ffd5eaf360e5c68ee46322906622296926fe5069c030c9474510a7456bdf15afa10256934600329e92697f222163154ae0c12c92894d8f5706df4d2ca98729b2c3d967ee7230e797e6815515b0029fcf138896daddb7d9ea7fc14ebd8f2d844ac074efe348ac58f3e0df3c16d0f7ed87ffdcebd7c6b4fd3cea9ce0a799e627deee1faa86f172a7542010a044176addce75fee1db3ba5f2bf3e3c22c1925b3c13eb189a570c9bab0eda85b2dfefd2f1e2044feb11bc3d123fc1f7bc8bde5dc81e292348309b85afb67000000ffff01050164026462026462536465762e762e696f3a6f3a3630383934313830383235362d34337674666e6465747337396b6635686163386965756a746f383833373636302e617070732e676f6f676c6575736572636f6e74656e742e636f6d01730c73675f3535313632333837380273626b6465762e762e696f3a6f3a3630383934313830383235362d34337674666e6465747337396b6635686163386965756a746f383833373636302e617070732e676f6f676c6575736572636f6e74",
			"31ca10d5-0195-54fa-9344-25fcd7072e01": "656e742e636f6d3a6461776e2e76616e616469756d40676d61696c2e636f6d0376697390016465762e762e696f3a6f3a3630383934313830383235362d34337674666e6465747337396b6635686163386965756a746f383833373636302e617070732e676f6f676c6575736572636f6e74656e742e636f6d3a6461776e2e76616e616469756d40676d61696c2e636f6d2c6465762e762e696f3a753a616c657866616e647269616e746f40676f6f676c652e636f6d005cb840d035a10ceaa1e9b7f238c057140000",
		},
		{ // Invalid characteristic value.
			"31ca10d5-0195-54fa-9344-25fcd7072e00": "7337396b6635686163386965756a746f383833373636302e617070732e676f6f676c6575736572636f6e74656e742e636f6d3a6461776e2e76616e616469756d40676d61696c2e636f6d0164026462026462536465762e762e696f3a6f3a3630383934313830383235362d34337674666e6465747337396b6635686163386965756a746f383833373636302e617070732e676f6f676c6575736572636f6e74656e742e636f6d01732c6c6973745f6c697374735f4a4d786659727655694d364f596d4f706b356b797350425048324e366b77536f480013c6163e316e5359a9d3ca49fbbc57140000",
		},
	}

	for i, test := range tests {
		cs := make(map[string][]byte)
		for k, v := range test {
			c, err := hex.DecodeString(v)
			if err != nil {
				t.Fatal(err)
			}
			cs[k] = c
		}

		if unpacked, err := unpackFromCharacteristics(cs); err == nil {
			for _, encoded := range unpacked {
				_, err := decodeAdInfo(encoded)
				if err == nil {
					t.Errorf("[%d]: expect an error; but got none", i)
				}
			}
		}
	}
}
