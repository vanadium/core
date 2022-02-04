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

var testCryptoAlgos = []keys.CryptoAlgo{keys.ECDSA256, keys.ED25519, keys.RSA2048}

func onePrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1 security.Principal)) {
	for _, k1 := range testCryptoAlgos {
		s1 := sectestdata.V23Signer(k1, sectestdata.V23KeySetA)
		logTest(t, "%v(%T)", name, s1.PublicKey())
		tester(t,
			sectest.NewPrincipalRootsOnly(t, s1),
		)
	}
}

func twoPrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1, p2 security.Principal)) {
	for _, k1 := range testCryptoAlgos {
		for _, k2 := range testCryptoAlgos {
			s1 := sectestdata.V23Signer(k1, sectestdata.V23KeySetA)
			s2 := sectestdata.V23Signer(k2, sectestdata.V23KeySetB)
			logTest(t, "%v(%T, %T)", name,
				s1.PublicKey(), s2.PublicKey())
			tester(t,
				sectest.NewPrincipalRootsOnly(t, s1),
				sectest.NewPrincipalRootsOnly(t, s2),
			)
		}
	}
}

func threePrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1, p2, p3 security.Principal)) {
	for _, k1 := range testCryptoAlgos {
		for _, k2 := range testCryptoAlgos {
			for _, k3 := range testCryptoAlgos {

				s1 := sectestdata.V23Signer(k1, sectestdata.V23KeySetA)
				s2 := sectestdata.V23Signer(k2, sectestdata.V23KeySetB)
				s3 := sectestdata.V23Signer(k3, sectestdata.V23KeySetC)
				logTest(t, "%v(%T, %T, %T)", name,
					s1.PublicKey(), s2.PublicKey(),
					s3.PublicKey())
				tester(t,
					sectest.NewPrincipalRootsOnly(t, s1),
					sectest.NewPrincipalRootsOnly(t, s2),
					sectest.NewPrincipalRootsOnly(t, s3),
				)
			}
		}
	}
}

func threeSignerTest(t *testing.T, name string, tester func(t *testing.T, s1, s2, s3 security.Signer)) {
	for _, k1 := range testCryptoAlgos {
		for _, k2 := range testCryptoAlgos {
			for _, k3 := range testCryptoAlgos {
				s1 := sectestdata.V23Signer(k1, sectestdata.V23KeySetA)
				s2 := sectestdata.V23Signer(k2, sectestdata.V23KeySetB)
				s3 := sectestdata.V23Signer(k3, sectestdata.V23KeySetC)
				logTest(t, "%v(%T, %T, %T)", name,
					s1.PublicKey(), s2.PublicKey(),
					s3.PublicKey())
				tester(t, s1, s2, s3)
			}
		}
	}
}

func fourPrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1, p2, p3, p4 security.Principal)) {
	for _, k1 := range testCryptoAlgos {
		for _, k2 := range testCryptoAlgos {
			for _, k3 := range testCryptoAlgos {
				for _, k4 := range testCryptoAlgos {
					s1 := sectestdata.V23Signer(k1, sectestdata.V23KeySetA)
					s2 := sectestdata.V23Signer(k2, sectestdata.V23KeySetB)
					s3 := sectestdata.V23Signer(k3, sectestdata.V23KeySetC)
					s4 := sectestdata.V23Signer(k4, sectestdata.V23KeySetD)
					logTest(t, "%v(%T, %T, %T, %T)", name,
						s1.PublicKey(), s2.PublicKey(),
						s3.PublicKey(), s4.PublicKey())
					tester(t,
						sectest.NewPrincipalRootsOnly(t, s1),
						sectest.NewPrincipalRootsOnly(t, s2),
						sectest.NewPrincipalRootsOnly(t, s3),
						sectest.NewPrincipalRootsOnly(t, s4),
					)
				}
			}
		}
	}
}

func fivePrincipalTest(t *testing.T, name string, tester func(t *testing.T, p1, p2, p3, p4, p5 security.Principal)) {
	for _, k1 := range testCryptoAlgos {
		for _, k2 := range testCryptoAlgos {
			for _, k3 := range testCryptoAlgos {
				for _, k4 := range testCryptoAlgos {
					for _, k5 := range testCryptoAlgos {
						s1 := sectestdata.V23Signer(k1, sectestdata.V23KeySetA)
						s2 := sectestdata.V23Signer(k2, sectestdata.V23KeySetB)
						s3 := sectestdata.V23Signer(k3, sectestdata.V23KeySetC)
						s4 := sectestdata.V23Signer(k4, sectestdata.V23KeySetD)
						s5 := sectestdata.V23Signer(k5, sectestdata.V23KeySetD)

						logTest(t, "%v(%T, %T, %T, %T, %T)", name,
							s1.PublicKey(), s2.PublicKey(),
							s3.PublicKey(), s4.PublicKey(),
							s5.PublicKey())
						tester(t,
							sectest.NewPrincipalRootsOnly(t, s1),
							sectest.NewPrincipalRootsOnly(t, s2),
							sectest.NewPrincipalRootsOnly(t, s3),
							sectest.NewPrincipalRootsOnly(t, s4),
							sectest.NewPrincipalRootsOnly(t, s5),
						)
					}
				}
			}
		}
	}
}
