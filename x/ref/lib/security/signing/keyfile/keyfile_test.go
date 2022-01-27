// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keyfile_test

import (
	"context"
	"fmt"
	"testing"

	"v.io/x/ref/lib/security/signing/keyfile"
	"v.io/x/ref/test/sectestdata"
)

func TestKeyFiles(t *testing.T) {
	ctx := context.Background()
	for _, typ := range sectestdata.SupportedKeyAlgos {
		key := sectestdata.V23PrivateKeyBytes(typ, sectestdata.V23keySetA)
		msg := fmt.Sprintf("X509/SSL key type %v", typ)
		testSigning(ctx, t, msg, key)

		key = sectestdata.X509PrivateKeyBytes(typ)
		msg = fmt.Sprintf("X509/SSL key type %v", typ)
		testSigning(ctx, t, msg, key)

		key = sectestdata.SSHPrivateKeyBytes(typ, sectestdata.SSHKeyPrivate)
		msg = fmt.Sprintf("SSH key type %v", typ)
		testSigning(ctx, t, msg, key)
	}
}

func testSigning(ctx context.Context, t *testing.T, msg string, keyBytes []byte) {
	svc := keyfile.NewSigningService()
	signer, err := svc.Signer(ctx, keyBytes, nil)
	if err != nil {
		t.Fatalf("failed to get signer for %v: %v", msg, err)
	}
	sig, err := signer.Sign([]byte("testing"), []byte("hello"))
	if err != nil {
		t.Fatalf("failed to sign message for %v: %v", msg, err)
	}
	if !sig.Verify(signer.PublicKey(), []byte("hello")) {
		t.Errorf("failed to verify signature for %v", msg)
	}
}
