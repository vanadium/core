// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ble

import (
	"math/big"
	"math/rand"
	"strings"
	"testing"

	idiscovery "v.io/x/ref/lib/discovery"
)

func TestServiceUuid(t *testing.T) {
	rand := rand.New(rand.NewSource(0))
	randString := func() string {
		p := make([]byte, rand.Intn(32))
		rand.Read(p)
		return string(p)
	}

	for i := 0; i < 10; i++ {
		interfaceName := randString()
		uuid := newServiceUuid(interfaceName)
		if !strings.HasPrefix(uuid.String(), "3dd1d5a8-") {
			t.Errorf("invalid uuid for %q: %v", interfaceName, uuid.String())
		}

		toggledUuid := idiscovery.Uuid(append([]byte(nil), uuid...))
		toggleServiceUuid(toggledUuid)

		uuidBits := new(big.Int).SetBytes([]byte(uuid))
		toggledUuidBits := new(big.Int).SetBytes([]byte(toggledUuid))

		if new(big.Int).Xor(uuidBits, toggledUuidBits).Bit(0) != 1 {
			t.Errorf("invalid toggled uuid for %v: %v", uuid.String(), toggledUuid.String())
		}
	}
}
