// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"crypto/rand"
	"encoding/hex"

	"v.io/v23/verror"
)

var (
	errAdIdSizeMismatch = verror.Register("v.io/v23/discovery.errAdIdSizeMismatch", verror.NoRetry, "id string size mismatch")

	zeroID = AdId{}
)

// IsValid reports whether the id is valid.
func (id AdId) IsValid() bool {
	return id != zeroID
}

// String returns the string corresponding to the id.
func (id AdId) String() string {
	return hex.EncodeToString(id[:])
}

// NewId returns a new random id.
func NewAdId() (AdId, error) {
	var id AdId
	if _, err := rand.Read(id[:]); err != nil {
		return zeroID, err
	}
	return id, nil
}

// Parse decodes the hexadecimal string into id.
func ParseAdId(s string) (AdId, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return zeroID, err
	}

	var id AdId
	if len(decoded) != len(id) {
		return zeroID, verror.New(errAdIdSizeMismatch, nil)
	}
	copy(id[:], decoded)
	return id, nil
}
