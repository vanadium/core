// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys

import (
	"context"
	"crypto"
	"encoding/pem"

	"v.io/x/ref/lib/security/keys/internal"
)

const (
	// IndirectionPrivateKeyPEMType is the PEM type used for the 'indirect'
	// PEM blocks intepreted by registrar.ParsePrivateKey.
	IndirectionPrivateKeyPEMType = "VANADIUM INDIRECT PRIVATE KEY"
	// IndirectionTypePEMHeader is a PEM header used to match an 'indirect'
	// PEM block to a particular parser.
	IndirectionTypePEMHeader = "Vanadium-Indirection-Type"
)

// MarshalFuncForIndirection returns a function that can be used to marshal
// its contents to an 'indirect' PEM block as understood by registrar.ParsePrivateKey
// for the specified indirectionType. The resulting PEM block will be matched
// by the function returned IndirectMatcherFunc for the same value of
// indirectionType.
func MarshalFuncForIndirection(indirectionType string) func(contents []byte) ([]byte, error) {
	return func(contents []byte) ([]byte, error) {
		return internal.EncodePEM(IndirectionPrivateKeyPEMType, contents, map[string]string{IndirectionTypePEMHeader: indirectionType})
	}
}

// IndirectPrivateKey represents the interface returned when an 'indirect'
// PEM block is encountered by registrar.ParsePrivateKey.
type IndirectPrivateKey interface {
	// Next returns either the key to be returned immediately, or data
	// to be parsed as per a normal PEM block.
	Next(ctx context.Context, passphrase []byte) (crypto.PrivateKey, []byte)
	String() string
}

// IndirectMatcherFunc returns a function that can be used to match an
// 'indirect' PEM block's IndirectionTypePEMHeader.
func IndirectMatcherFunc(indirectionType string) func(b *pem.Block) bool {
	return func(b *pem.Block) bool {
		return b.Headers[IndirectionTypePEMHeader] == indirectionType
	}
}
