// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"encoding/base64"
	"fmt"

	"v.io/v23/security"
	"v.io/v23/vom"
)

// DecodeBlessingsBase64 decodes blessings from the supplied base64
// url encoded string.
func DecodeBlessingsBase64(encoded string) (security.Blessings, error) {
	var b security.Blessings
	if err := base64urlVomDecode(encoded, &b); err != nil {
		return security.Blessings{}, fmt.Errorf("failed to decode %v: %v", encoded, err)
	}
	return b, nil
}

func base64urlVomDecode(s string, i interface{}) error {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return vom.Decode(b, i)
}

// EncodeBlessingsBase64 encodes the supplied blessings as a base 64
// url encoded string.
func EncodeBlessingsBase64(blessings security.Blessings) (string, error) {
	if blessings.IsZero() {
		return "", fmt.Errorf("no blessings found")
	}
	str, err := base64urlVomEncode(blessings)
	if err != nil {
		return "", fmt.Errorf("base64url-vom encoding failed: %v", err)
	}
	return str, nil
}

func base64urlVomEncode(i interface{}) (string, error) {
	buf, err := vom.Encode(i)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}
