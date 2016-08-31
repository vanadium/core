// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package restsigner_test

import (
	"math/big"
	"testing"

	"v.io/v23/security"
	"v.io/x/ref/services/internal/restsigner"
	signer "v.io/x/ref/services/internal/restsigner/signer/v1"
)

func TestDecode(t *testing.T) {
	// To generate encodedKey and encodedSig run the binary in v.io/x/ref/services/identity/internal/rest_signer_test
	encodedKey := &signer.PublicKey{Base64: "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEx8xywmgl2_UmDUoJxrh2N9pAij7jg1kIqruKpnT6SNtcNubCG_PgdpWqiVLp3zBWlw1T3F2ecy4iGpi5N4Yj-A=="}
	encodedSig := &signer.VSignature{R: "90128808689861327833210881969781001621382090117447023854233028840694123302875", S: "102696248968928040866906648206566376772954871370602978407028885005693672370943"}

	key, err := restsigner.DecodePublicKey(encodedKey)
	if err != nil {
		t.Fatal(err)
	}

	s := security.NewECDSASigner(key, func(message []byte) (r, s *big.Int, err error) {
		return restsigner.DecodeSignature(encodedSig)
	})
	sig, err := s.Sign([]byte("purpose"), []byte("message"))
	if err != nil {
		t.Fatal(err)
	}
	if !sig.Verify(s.PublicKey(), []byte("message")) {
		t.Fatal("Signature does not verify")
	}
}
