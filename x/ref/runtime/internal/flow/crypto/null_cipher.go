// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crypto

// NullControlCipher is a cipher that does nothing.
type NullControlCipher struct{}

func (NullControlCipher) MACSize() int           { return 0 }
func (NullControlCipher) Seal(data []byte) error { return nil }
func (NullControlCipher) Open(data []byte) bool  { return true }
func (NullControlCipher) Encrypt(data []byte)    {}
func (NullControlCipher) Decrypt(data []byte)    {}
func (NullControlCipher) ChannelBinding() []byte { return nil }

type disabledControlCipher struct {
	NullControlCipher
	macSize int
}

func (c *disabledControlCipher) MACSize() int { return c.macSize }

// NewDisabledControlCipher returns a cipher that has the correct MACSize, but
// encryption and decryption are disabled.
func NewDisabledControlCipher(c ControlCipher) ControlCipher {
	return &disabledControlCipher{macSize: c.MACSize()}
}
