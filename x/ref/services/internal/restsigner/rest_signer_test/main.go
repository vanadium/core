// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"encoding/base64"
	"fmt"
	"math/big"

	"v.io/x/ref/services/internal/restsigner"
)

func main() {
	signer, err := restsigner.NewRestSigner()
	if err != nil {
		fmt.Printf("NewRestSigner error: %v\n", err)
		return
	}
	der, err := signer.PublicKey().MarshalBinary()
	if err != nil {
		fmt.Printf("Failed to marshal public key: %v\n", err)
		return
	}
	sig, err := signer.Sign([]byte("purpose"), []byte("message"))
	if err != nil {
		fmt.Printf("Sign error: %v\n", err)
		return
	}
	ok := sig.Verify(signer.PublicKey(), []byte("message"))
	fmt.Printf("PublicKey: %v\n", base64.URLEncoding.EncodeToString(der))
	fmt.Printf("R: %v\n", big.NewInt(0).SetBytes(sig.R))
	fmt.Printf("S: %v\n", big.NewInt(0).SetBytes(sig.S))
	fmt.Printf("Verified: %v\n", ok)
}
