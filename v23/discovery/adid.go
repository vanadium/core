// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

var zeroID = AdId{}

// IsValid reports whether the id is valid.
func (id AdId) IsValid() bool {
	return id != zeroID
}

// String returns the string corresponding to the id.
func (id AdId) String() string {
	return hex.EncodeToString(id[:])
}

// NewId returns a new random id.
//nolint:golint // API change required.
func NewAdId() (AdId, error) {
	var id AdId
	if _, err := rand.Read(id[:]); err != nil {
		return zeroID, err
	}
	return id, nil
}

// Parse decodes the hexadecimal string into id.
//nolint:golint // API change required.
func ParseAdId(s string) (AdId, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return zeroID, err
	}

	var id AdId
	if len(decoded) != len(id) {
		return zeroID, fmt.Errorf("id string size mismatch")
	}
	copy(id[:], decoded)
	return id, nil
}
