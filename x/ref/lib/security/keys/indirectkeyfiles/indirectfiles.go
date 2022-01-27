// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package indirectkeyfiles provides support for key files whose contents
// refer to another keyfile.
package indirectkeyfiles

import (
	"context"
	"crypto"
	"encoding/pem"
	"fmt"
	"os"

	"v.io/x/ref/lib/security/keys"
)

// MustRegister is like Register but panics on error.
func MustRegister(r *keys.Registrar) {
	if err := Register(r); err != nil {
		panic(err)
	}
}

const indirectHeaderValue = "filename"

// Register registers the required functions for handling indirect key files
// via the x/ref/security/keys package.
func Register(r *keys.Registrar) error {
	r.RegisterIndirectPrivateKeyParser(parsePrivateKeyFile, indirectHeaderValue)
	return nil
}

// MarshalPrivateKey marshals a filename into the PEM block suitable
// for parsing by this package.
var MarshalPrivateKey = keys.MarshalFuncForIndirectType(indirectHeaderValue)

type indirection struct {
	filename string
	data     []byte
}

// Next implements keys.Indirection.
func (i *indirection) Next(ctx context.Context) (crypto.PrivateKey, []byte) {
	return nil, i.data
}

// String implements keys.Indirection.
func (i *indirection) String() string {
	return fmt.Sprintf("filename: %v", i.filename)
}

func parsePrivateKeyFile(pem *pem.Block) (crypto.PrivateKey, error) {
	filename := string(pem.Bytes)
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read filename in %q: %v", pem.Type, err)
	}
	return &indirection{filename: filename, data: data}, nil
}
