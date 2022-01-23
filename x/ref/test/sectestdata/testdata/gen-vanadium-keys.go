// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"

	"v.io/x/ref/lib/security"
	"v.io/x/ref/test/sectestdata"
)

func main() {
	for _, kd := range sectestdata.SupportedKeyTypes {
		for _, set := range []string{"-a-", "-b-"} {
			pk, err := security.NewPrivateKey(security.KeyType(kd))
			if err != nil {
				panic(err)
			}
			data, err := x509.MarshalPKCS8PrivateKey(pk)
			if err != nil {
				panic(err)
			}
			pemKey := &pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: data,
			}
			encoded := &bytes.Buffer{}
			if err := pem.Encode(encoded, pemKey); err != nil {
				panic(err)
			}
			if err := os.WriteFile("v23-private"+set+kd.String()+".key", encoded.Bytes(), 0600); err != nil {
				panic(err)
			}
		}
	}
}
