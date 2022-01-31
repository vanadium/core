// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"encoding/pem"
	"fmt"
)

type ImportOptions struct {
	KeyBytes       []byte
	OrigPassphrase []byte
	NewPassphrase  []byte
	KeyFilename    string
	UseAgent       bool
}

func (o *ImportOptions) PrivateKeyBytes(keyBytes []byte, origPassphrase, newPassphrase []byte) {
	o.KeyBytes = keyBytes
	o.OrigPassphrase = origPassphrase
	o.NewPassphrase = newPassphrase
}

func (o *ImportOptions) PrivateKeyFile(filename string) {
	o.KeyFilename = filename
}

func (o *ImportOptions) UsingAgent(v bool) {
	o.UseAgent = v
}

func ZeroPassphrases(passphrases ...[]byte) {
	for _, pp := range passphrases {
		for i := range pp {
			pp[i] = 0x0
		}
	}
}

func EncodePEM(typ string, der []byte, headers map[string]string) ([]byte, error) {
	var out bytes.Buffer
	block := pem.Block{
		Type:    typ,
		Headers: headers,
		Bytes:   der,
	}
	if err := pem.Encode(&out, &block); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func DecodePEM(pemBytes []byte, typ string) (*pem.Block, error) {
	var block *pem.Block
	for {
		block, pemBytes = pem.Decode(pemBytes)
		if block == nil {
			return nil, fmt.Errorf("failed to find %v in any PEM block", typ)
		}
		if block.Type == typ {
			return block, nil
		}
	}
}
