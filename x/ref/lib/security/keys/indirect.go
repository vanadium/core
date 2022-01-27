// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys

import (
	"bytes"
	"context"
	"crypto"
	"encoding/pem"
)

const (
	PrivateKeyPEMType = "VANADIUM INDIRECT PRIVATE KEY"
	pemHeader         = "Vanadium-Indirection-Type"
)

func marshalIndirectType(typ string, contents []byte) ([]byte, error) {
	block := pem.Block{
		Type:    PrivateKeyPEMType,
		Headers: map[string]string{pemHeader: typ},
		Bytes:   contents,
	}
	var out bytes.Buffer
	if err := pem.Encode(&out, &block); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func MarshalFuncForIndirectType(typ string) func(contents []byte) ([]byte, error) {
	return func(contents []byte) ([]byte, error) {
		return marshalIndirectType(typ, contents)
	}
}

type IndirectPrivateKey interface {
	Next(ctx context.Context) (crypto.PrivateKey, []byte)
	String() string
}

func MatcherFunc(typ string) func(b *pem.Block) bool {
	return func(b *pem.Block) bool {
		return b.Headers[pemHeader] == typ
	}
}
