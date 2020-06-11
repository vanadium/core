// Package keyfile provides a signing service that uses files to store
// keys. Currently .pem and ssh private key file formats are supported.
package keyfile

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/signing"
)

// TODO(cnicolaou): this implementation anticipates forthcoming changes
//   to use context.Context and to simplify verror. The TODO is to transition
//   to their replacements, for now use context.Context and fmt.Errorf
//   directly in some cases.

type keyfile struct {
}

func NewSigningService() signing.Service {
	return &keyfile{}
}

func determineSigner(key interface{}) (security.Signer, error) {
	switch v := key.(type) {
	case *ecdsa.PrivateKey:
		return security.NewInMemoryECDSASigner(v), nil
	case *ed25519.PrivateKey:
		return nil, fmt.Errorf("unsupported signing key type %T", key)
	default:
		return nil, fmt.Errorf("unsupported signing key type %T", key)
	}
}

// Signer implements v.io/ref/lib/security.SigningService.
// The suffix for keyFile determines how the file is parsed:
//   - .pem for PEM files
//   - .ssh for ssh private key files and .ssh.pub for public keys.
func (kf *keyfile) Signer(ctx context.Context, keyFile string, passphrase []byte) (security.Signer, error) {
	f, err := os.Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	switch {
	case strings.HasSuffix(keyFile, ".pem"):
		key, err := internal.LoadPEMPrivateKey(f, passphrase)
		if err != nil {
			return nil, err
		}
		return determineSigner(key)
	case strings.HasSuffix(keyFile, ".ssh"):
		sshPemBytes, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("failed to read all of %v: %v", keyFile, err)
		}
		// TODO(cnicolaou): consider support for storing the name/comment of
		//   the ssh key in the .ssh file rather than the file itself when the
		//   key is stored exclusively in the ssh-agent.
		var sshKey interface{}
		if passphrase == nil {
			sshKey, err = ssh.ParseRawPrivateKey(sshPemBytes)
		} else {
			sshKey, err = ssh.ParseRawPrivateKeyWithPassphrase(sshPemBytes, passphrase)
		}
		switch {
		case err == nil:
			return determineSigner(sshKey)
		case errors.Is(err, &ssh.PassphraseMissingError{}):
			return nil, verror.New(internal.ErrPassphraseRequired, nil)
		case err == x509.IncorrectPasswordError:
			return nil, verror.New(internal.ErrBadPassphrase, nil)
		default:
			return nil, fmt.Errorf("failed to parse SSH key from %v: %v", keyFile, err)
		}
	}
	return nil, fmt.Errorf("unrecognised file suffix: %v (must be one of .pem or .ssh)", keyFile)
}

// Close implements v.io/ref/lib/security.SigningService.
func (kf *keyfile) Close(ctx context.Context) error {
	return nil
}
