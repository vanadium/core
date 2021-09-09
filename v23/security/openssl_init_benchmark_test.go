// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

package security_test

import (
	"sync"

	"v.io/v23/security"
)

var (
	opensslECDSAKey      *bmkey
	opensslED25519Key    *bmkey
	opensslRSA2048Key    *bmkey
	opensslRSA4096Key    *bmkey
	opensslBenchmarkInit sync.Once
)

func initOpenSSLBenchmarks() {
	opensslBenchmarkInit.Do(func() {
		opensslRSA2048Key = newRSABenchmarkKey(2048, security.NewOpenSSLRSASigner)
		opensslRSA4096Key = newRSABenchmarkKey(4096, security.NewOpenSSLRSASigner)
		opensslECDSAKey = newECDSABenchmarkKey(security.NewOpenSSLECDSASigner)
		opensslED25519Key = newED25519BenchmarkKey(security.NewOpenSSLED25519Signer)
	})
}
