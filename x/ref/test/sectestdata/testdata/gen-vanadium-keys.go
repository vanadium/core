// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/x509"
	"os"

	"v.io/x/ref/lib/security"
)

func main() {
	for _, kd := range []security.KeyType{
		security.ECDSA256,
		security.ECDSA384,
		security.ECDSA521,
		security.ED25519,
		security.RSA2048,
		security.RSA4096,
	} {
		for _, set := range []string{"-a-", "-b-"} {
			pk, err := security.NewPrivateKey(kd)
			if err != nil {
				panic(err)
			}
			data, err := x509.MarshalPKCS8PrivateKey(pk)
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile("v23-private"+set+kd.String()+".key", data, 0600); err != nil {
				panic(err)
			}
		}
	}
}
