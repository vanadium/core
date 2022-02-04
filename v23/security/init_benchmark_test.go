// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

var (
	ecdsaKey   *bmkey
	ed25519Key *bmkey
	rsa2048Key *bmkey

	ecdsa256SignerA security.Signer
	ed25519SignerA  security.Signer
	rsa2048SignerA  security.Signer
	rsa4096SignerA  security.Signer
	ecdsa256SignerB security.Signer
	ed25519SignerB  security.Signer
	rsa2048SignerB  security.Signer
	rsa4096SignerB  security.Signer
)

var (
	purpose, message []byte
)

func init() {
	purpose, message = sectest.GenPurposeAndMessage(5, 100)

}

type bmkey struct {
	signer    security.Signer
	signature security.Signature
}

func newBenchmarkKey(signer security.Signer) *bmkey {
	signature, err := signer.Sign(purpose, message)
	if err != nil {
		panic(err)
	}
	return &bmkey{signer, signature}
}

func init() {
	ecdsa256SignerA = sectestdata.V23Signer(keys.ECDSA256, sectestdata.V23KeySetA)
	ed25519SignerA = sectestdata.V23Signer(keys.ED25519, sectestdata.V23KeySetA)
	rsa2048SignerA = sectestdata.V23Signer(keys.RSA2048, sectestdata.V23KeySetA)
	rsa4096SignerA = sectestdata.V23Signer(keys.RSA4096, sectestdata.V23KeySetA)

	ecdsa256SignerB = sectestdata.V23Signer(keys.ECDSA256, sectestdata.V23KeySetB)
	ed25519SignerB = sectestdata.V23Signer(keys.ED25519, sectestdata.V23KeySetB)
	rsa2048SignerB = sectestdata.V23Signer(keys.RSA2048, sectestdata.V23KeySetB)
	rsa4096SignerB = sectestdata.V23Signer(keys.RSA4096, sectestdata.V23KeySetB)

	ecdsaKey = newBenchmarkKey(ecdsa256SignerA)
	ed25519Key = newBenchmarkKey(ed25519SignerA)
	rsa2048Key = newBenchmarkKey(rsa2048SignerA)
}
