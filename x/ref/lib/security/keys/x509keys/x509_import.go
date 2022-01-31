// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509keys

import (
	"context"
	"crypto/x509"
	"fmt"

	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
	"v.io/x/ref/lib/security/keys/internal"
)

// ImportOption represents an option to MarshalForImport.
type ImportOption func(o *internal.ImportOptions)

// ImportPrivateKeyBytes will result in the supplied bytes being used
// as the private key, that is, those bytes will be copied to the
// Vanadium principal. If newPassphrase is provided the copied key will
// be encrypted in pkcs8 format.
func ImportPrivateKeyBytes(keyBytes []byte, origPassphrase, newPassphrase []byte) ImportOption {
	return func(o *internal.ImportOptions) {
		o.PrivateKeyBytes(keyBytes, origPassphrase, newPassphrase)
	}
}

// ImportPrivateKeyFile will result in the private key in the specified
// file being used. The Vanadium principal will refer to that file and
// not copy it.
func ImportPrivateKeyFile(filename string) ImportOption {
	return func(o *internal.ImportOptions) {
		o.PrivateKeyFile(filename)
	}
}

// MarshalForImport will marshal the supplied public and private keys
// according to the supplied option which specifies how the private key
// is to imported.
func MarshalForImport(ctx context.Context, publicKeyBytes []byte, option ImportOption) (importedPublicKeyBytes, importedPrivateKeyBytes []byte, err error) {
	opts := internal.ImportOptions{}
	option(&opts)
	defer internal.ZeroPassphrases(opts.OrigPassphrase, opts.NewPassphrase)

	// Read the first certificate and then re-encode just that one cert.
	cert, err := parseFirstCertificate(publicKeyBytes)
	if err != nil {
		return nil, nil, err
	}
	publicKeyBytes, err = internal.EncodePEM("CERTIFICATE", cert.Raw, nil)

	if filename := opts.KeyFilename; len(filename) > 0 {
		privKeyBytes, err := indirectkeyfiles.MarshalPrivateKey([]byte(filename))
		if err != nil {
			return nil, nil, err
		}
		return publicKeyBytes, privKeyBytes, nil
	}

	if keyBytes := opts.KeyBytes; len(keyBytes) > 0 {
		privKeyBytes, err := importPrivateKeyBytes(ctx, keyBytes,
			opts.OrigPassphrase, opts.NewPassphrase)
		return publicKeyBytes, privKeyBytes, err
	}

	return nil, nil, fmt.Errorf("no options were specified for how to import the private key")
}

func parseFirstCertificate(pemBytes []byte) (*x509.Certificate, error) {
	block, err := internal.DecodePEM(pemBytes, "CERTIFICATE")
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(block.Bytes)
}

// Make the key functions local to this package available to this package.
var keyRegistrar = keys.NewRegistrar()

func init() {
	keys.MustRegister(keyRegistrar)
}

func importPrivateKeyBytes(ctx context.Context, keyBytes, origPassphrase, newPassphrase []byte) ([]byte, error) {
	if len(newPassphrase) == 0 {
		// downstream code can cope with cleartext or encrypted keys.
		return keyBytes, nil
	}
	// We need to encrypt the imported key bytes, so obtain the original
	// key (which may also be encrypted) and then reencrypt it.
	privKey, err := keyRegistrar.ParsePrivateKey(ctx, keyBytes, origPassphrase)
	if err != nil {
		return nil, err
	}
	// Note that the encrypted key will always be in PCKS8 format.
	return keys.MarshalPKCS8PrivateKey(privKey, newPassphrase)
}
