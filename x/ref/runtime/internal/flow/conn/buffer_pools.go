// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"sync"
)

const (
	// estimate of how the overhead of the message header fields other
	// than the payloads.
	estimatedMessageOverhead = 256

	// max of gcmTagSize (16) and box.Overhead (16)
	maxCipherOverhead = 16

	// The plaintext message pipe buffer needs to allow for the overhead
	// of the message itself header fields as well as its payload.
	plaintextBufferSize = defaultMtu + estimatedMessageOverhead

	// The ciphertext buffer needs to allow for the cipher overhead also.
	ciphertextBufferSize = plaintextBufferSize + maxCipherOverhead
)

var (
	// intermediate buffers used by the message pipe for compression/decompression.
	messagePipePool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, ciphertextBufferSize)
			// Return a pointer to the slice to avoid unnecessary allocations
			// when returning buffers to the pool.
			return &b
		},
	}
)
