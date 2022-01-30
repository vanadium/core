// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !openssl
// +build !openssl

package sectestdata

import (
	"v.io/x/ref/lib/security/keys"
)

var (
	publicSignerKeyTypes = map[keys.CryptoAlgo]string{}
)

func init() {
	publicSignerKeyTypes[keys.ECDSA256] = "*security.ecdsaPublicKey"
	publicSignerKeyTypes[keys.ECDSA384] = "*security.ecdsaPublicKey"
	publicSignerKeyTypes[keys.ECDSA521] = "*security.ecdsaPublicKey"
	publicSignerKeyTypes[keys.ED25519] = "*security.ed25519PublicKey"
	publicSignerKeyTypes[keys.RSA2048] = "*security.rsaPublicKey"
	publicSignerKeyTypes[keys.RSA4096] = "*security.rsaPublicKey"
}
