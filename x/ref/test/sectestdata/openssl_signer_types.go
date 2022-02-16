// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
// +build openssl

package sectestdata

import (
	"v.io/x/ref/lib/security/keys"
)

var (
	publicSignerKeyTypes = map[keys.CryptoAlgo]string{}
)

func init() {
	publicSignerKeyTypes[keys.ECDSA256] = "*security.opensslECDSAPublicKey"
	publicSignerKeyTypes[keys.ECDSA384] = "*security.opensslECDSAPublicKey"
	publicSignerKeyTypes[keys.ECDSA521] = "*security.opensslECDSAPublicKey"
	publicSignerKeyTypes[keys.ED25519] = "*security.opensslED25519PublicKey"
	publicSignerKeyTypes[keys.RSA2048] = "*security.opensslRSAPublicKey"
	publicSignerKeyTypes[keys.RSA4096] = "*security.opensslRSAPublicKey"
}
