// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cipher

import (
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
)

// GenerateKey is the same as box.GenerateKey but uses X25519 rather than
// ScalarBaseMult and incudes additional error checking.
func GenerateKey(rand io.Reader) (publicKey, privateKey *[32]byte, err error) {
	publicKey = new([32]byte)
	privateKey = new([32]byte)
	_, err = io.ReadFull(rand, privateKey[:])
	if err != nil {
		publicKey = nil
		privateKey = nil
		return
	}
	pk, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		publicKey = nil
		privateKey = nil
		return
	}
	n := copy(publicKey[:], pk)
	if n != len(publicKey) {
		err = fmt.Errorf("public key is of the wrong size: %v not %v", n, len(publicKey))
		publicKey = nil
		privateKey = nil
		return
	}
	return
}
