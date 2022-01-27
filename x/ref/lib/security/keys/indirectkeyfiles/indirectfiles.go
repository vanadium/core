// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package indirectkeyfiles

import (
	"crypto"
	"encoding/pem"
	"fmt"
	"os"

	"v.io/x/ref/lib/security/keys"
)

func MustRegister(r *keys.Registrar) {
	if err := Register(r); err != nil {
		panic(err)
	}
}

func Register(r *keys.Registrar) error {
	r.RegisterIndirectPrivateKeyParser(ParsePrivateKeyFile, "filename")
	return nil
}

var MarshalPrivateKey = keys.MarshalFuncForIndirectType("filename")

type indirection struct {
	filename string
	data     []byte
}

func (i *indirection) Next() (crypto.PrivateKey, []byte) {
	return nil, i.data
}

func (i *indirection) String() string {
	return fmt.Sprintf("filename: %v", i.filename)
}

func ParsePrivateKeyFile(pem *pem.Block) (crypto.PrivateKey, error) {
	filename := string(pem.Bytes)
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read filename in %q: %v", pem.Type, err)
	}
	return &indirection{filename: filename, data: data}, nil
}
