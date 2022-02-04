// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"testing"

	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

const logTestTypes = false

func logTest(t *testing.T, format string, args ...interface{}) {
	if !logTestTypes {
		return
	}
	t.Logf(format, args...)
}

func onePrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1 security.Principal)) {
	for _, first := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
		firstSigner := sectestdata.V23Signer(first, sectestdata.V23KeySetA)
		logTest(t, "%v(%T)", name, firstSigner.PublicKey())
		tester(t,
			sectest.NewPrincipal(t, firstSigner, nil, &sectest.Roots{}),
		)
	}
}

func twoPrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1, p2 security.Principal)) {
	for _, first := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
		for _, second := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
			firstSigner := sectestdata.V23Signer(first, sectestdata.V23KeySetA)
			secondSigner := sectestdata.V23Signer(second, sectestdata.V23KeySetB)
			logTest(t, "%v(%T, %T)", name,
				firstSigner.PublicKey(), secondSigner.PublicKey())
			tester(t,
				sectest.NewPrincipal(t, firstSigner, nil, &sectest.Roots{}),
				sectest.NewPrincipal(t, secondSigner, nil, &sectest.Roots{}),
			)
		}
	}
}

func threePrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1, p2, p3 security.Principal)) {
	for _, first := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
		for _, second := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
			for _, third := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {

				firstSigner := sectestdata.V23Signer(first, sectestdata.V23KeySetA)
				secondSigner := sectestdata.V23Signer(second, sectestdata.V23KeySetB)
				thirdSigner := sectestdata.V23Signer(third, sectestdata.V23KeySetC)
				logTest(t, "%v(%T, %T, %T)", name,
					firstSigner.PublicKey(), secondSigner.PublicKey(),
					thirdSigner.PublicKey())
				tester(t,
					sectest.NewPrincipal(t, firstSigner, nil, &sectest.Roots{}),
					sectest.NewPrincipal(t, secondSigner, nil, &sectest.Roots{}),
					sectest.NewPrincipal(t, thirdSigner, nil, &sectest.Roots{}),
				)
			}
		}
	}
}

func threeSignerTest(t *testing.T, name string, tester func(t *testing.T, s1, s2, s3 security.Signer)) {
	for _, first := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
		for _, second := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
			for _, third := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
				firstSigner := sectestdata.V23Signer(first, sectestdata.V23KeySetA)
				secondSigner := sectestdata.V23Signer(second, sectestdata.V23KeySetB)
				thirdSigner := sectestdata.V23Signer(third, sectestdata.V23KeySetC)
				logTest(t, "%v(%T, %T, %T)", name,
					firstSigner.PublicKey(), secondSigner.PublicKey(),
					thirdSigner.PublicKey())
				tester(t, firstSigner, secondSigner, thirdSigner)
			}
		}
	}
}

func fourPrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1, p2, p3, p4 security.Principal)) {
	for _, first := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
		for _, second := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
			for _, third := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
				for _, fourth := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
					firstSigner := sectestdata.V23Signer(first, sectestdata.V23KeySetA)
					secondSigner := sectestdata.V23Signer(second, sectestdata.V23KeySetB)
					thirdSigner := sectestdata.V23Signer(third, sectestdata.V23KeySetC)
					fourthSigner := sectestdata.V23Signer(fourth, sectestdata.V23KeySetD)
					logTest(t, "%v(%T, %T, %T, %T)", name,
						firstSigner.PublicKey(), secondSigner.PublicKey(),
						thirdSigner.PublicKey(), fourthSigner.PublicKey())
					tester(t,
						sectest.NewPrincipal(t, firstSigner, nil, &sectest.Roots{}),
						sectest.NewPrincipal(t, secondSigner, nil, &sectest.Roots{}),
						sectest.NewPrincipal(t, thirdSigner, nil, &sectest.Roots{}),
						sectest.NewPrincipal(t, fourthSigner, nil, &sectest.Roots{}),
					)
				}
			}
		}
	}
}

func fivePrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1, p2, p3, p4, p5 security.Principal)) {
	for _, first := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
		for _, second := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
			for _, third := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
				for _, fourth := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
					for _, five := range []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048} {
						firstSigner := sectestdata.V23Signer(first, sectestdata.V23KeySetA)
						secondSigner := sectestdata.V23Signer(second, sectestdata.V23KeySetB)
						thirdSigner := sectestdata.V23Signer(third, sectestdata.V23KeySetC)
						fourthSigner := sectestdata.V23Signer(fourth, sectestdata.V23KeySetD)
						fifthSigner := sectestdata.V23Signer(five, sectestdata.V23KeySetD)

						logTest(t, "%v(%T, %T, %T, %T, %T)", name,
							firstSigner.PublicKey(), secondSigner.PublicKey(),
							thirdSigner.PublicKey(), fourthSigner.PublicKey(),
							fifthSigner.PublicKey())
						tester(t,
							sectest.NewPrincipal(t, firstSigner, nil, &sectest.Roots{}),
							sectest.NewPrincipal(t, secondSigner, nil, &sectest.Roots{}),
							sectest.NewPrincipal(t, thirdSigner, nil, &sectest.Roots{}),
							sectest.NewPrincipal(t, fourthSigner, nil, &sectest.Roots{}),
							sectest.NewPrincipal(t, fifthSigner, nil, &sectest.Roots{}),
						)
					}
				}
			}
		}
	}
}
