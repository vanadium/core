// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"

	"v.io/v23/security"
	"v.io/v23/vom"
)

// Macaroon encapsulates an arbitrary slice of data signed with a Private Key.
// Term borrowed from http://research.google.com/pubs/pub41892.html.
type Macaroon string

type MacaroonMessage struct {
	Data []byte
	Sig  security.Signature
}

// NewMacaroon creates an opaque token that encodes "data".
//
// Input can be extracted from the returned token only if the Signature is
// valid.
func NewMacaroon(principal security.Principal, data []byte) (Macaroon, error) {
	if data == nil {
		data = []byte{}
	}
	sig, err := principal.Sign(data)
	if err != nil {
		return Macaroon(""), err
	}
	v, err := vom.Encode(MacaroonMessage{Data: data, Sig: sig})
	if err != nil {
		return Macaroon(""), err
	}
	return Macaroon(b64encode(v)), nil
}

// Decode returns the input if the macaroon was signed by the current
// principal.
func (m Macaroon) Decode(principal security.Principal) (input []byte, err error) {
	decoded, err := b64decode(string(m))
	if err != nil {
		return nil, err
	}
	var msg MacaroonMessage
	if err := vom.Decode(decoded, &msg); err != nil {
		return nil, err
	}
	if msg.Sig.Verify(principal.PublicKey(), msg.Data) {
		return msg.Data, nil
	}
	return nil, fmt.Errorf("invalid macaroon, Signature does not match")
}
