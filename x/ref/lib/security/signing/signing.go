// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package signing

import (
	"context"

	"v.io/v23/security"
)

// Service defines the interface for a signing service.
type Service interface {
	// Signer returns a security.Signer for the specified key, which will
	// generally be referred to as a filename. Credentials
	// will generally be a passphrase for accessing the key.
	Signer(ctx context.Context, key string, credentials []byte) (security.Signer, error)

	// Close releases/closes all resources associated with the service instance.
	Close(ctx context.Context) error
}
