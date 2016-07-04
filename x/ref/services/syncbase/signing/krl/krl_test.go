// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package krl tests the key revocation list package.
package krl_test

import "runtime"
import "testing"
import "time"

import "v.io/x/ref/services/syncbase/signing/krl"

// checkKeysNotRevoked() checks that key[start:] have not been revoked.  (The
// start index is passed, rather than expecting the called to sub-slice, so
// that error messages refer to the expected index.)
func checkKeysNotRevoked(t *testing.T, krl *krl.KRL, start int, key [][]byte, now time.Time) {
	_, _, callerLine, _ := runtime.Caller(1)
	year := 365 * 24 * time.Hour
	for i := start; i != len(key); i++ {
		revoked := krl.RevocationTime(key[i])
		if revoked.Before(now.Add(year)) {
			t.Errorf("line %d: unrevoked key[%d]=%v has revocation time %v, which is not far enough in the future", callerLine, i, key[i], revoked)
		}
	}
}

func TestKRL(t *testing.T) {
	now := time.Now()
	key := [][]byte{
		[]byte{0x00, 0x01, 0x02, 0x3},
		[]byte{0x04, 0x05, 0x06, 0x7},
		[]byte{0x08, 0x09, 0x0a, 0xb}}
	var revoked time.Time

	krl := krl.New()

	checkKeysNotRevoked(t, krl, 0, key, now)

	krl.Revoke(key[0], now)
	if revoked = krl.RevocationTime(key[0]); !revoked.Equal(now) {
		t.Errorf("unrevoked key %v has revocation time %v, but expected %v", key[0], revoked, now)
	}
	checkKeysNotRevoked(t, krl, 1, key, now)

	krl.Revoke(key[1], now)
	if revoked = krl.RevocationTime(key[0]); !revoked.Equal(now) {
		t.Errorf("unrevoked key %v has revocation time %v, but expected %v", key[0], revoked, now)
	}
	if revoked = krl.RevocationTime(key[1]); !revoked.Equal(now) {
		t.Errorf("unrevoked key %v has revocation time %v, but expected %v", key[1], revoked, now)
	}
	checkKeysNotRevoked(t, krl, 2, key, now)
}
