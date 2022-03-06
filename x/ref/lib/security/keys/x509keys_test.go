// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys_test

import (
	"context"
	"testing"

	"v.io/x/ref/test/sectestdata"
)

func TestParsingX509Keys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		testParsing(
			ctx, t, kt,
			sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Private),
			sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Encrypted),
			sectestdata.X509PublicKeyBytes(kt),
		)
	}
}

func TestPasswordsX509Keys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		testPasswordEncryption(
			ctx, t, kt,
			sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Private),
			sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Encrypted),
			sectestdata.X509PublicKeyBytes(kt),
		)
	}
}
