package signing

import (
	"context"

	"v.io/v23/security"
)

// Service defines the interface for a signing service.
type Service interface {
	// Signer returns a security.Signer for the specified key. Credentials may contain
	// the secret key or be a passphrase for accessing the key.
	Signer(ctx context.Context, keyName string, credentials []byte) (security.Signer, error)

	// Close releases/closes all resources associated with the service instance.
	Close(ctx context.Context) error
}
