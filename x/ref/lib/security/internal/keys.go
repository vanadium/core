package internal

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"v.io/v23/verror"
)

const (
	pkgPath                = "v.io/x/ref/lib/security/internal"
	ecPrivateKeyPEMType    = "EC PRIVATE KEY"
	ecPublicKeyPEMType     = "EC PUBLIC KEY"
	pkcs8PrivateKeyPEMType = "PRIVATE KEY"
)

var (
	// ErrBadPassphrase is a possible return error from LoadPEMPrivateKey()
	ErrBadPassphrase = verror.Register(pkgPath+".errBadPassphrase", verror.NoRetry, "{1:}{2:} passphrase incorrect for decrypting private key{:_}")
	// ErrPassphraseRequired is a possible return error from LoadPEMPrivateKey()
	ErrPassphraseRequired = verror.Register(pkgPath+".errPassphraseRequired", verror.NoRetry, "{1:}{2:} passphrase required for decrypting private key{:_}")

	errNoPEMKeyBlock       = verror.Register(pkgPath+".errNoPEMKeyBlock", verror.NoRetry, "{1:}{2:} no PEM key block read{:_}")
	errPEMKeyBlockBadType  = verror.Register(pkgPath+".errPEMKeyBlockBadType", verror.NoRetry, "{1:}{2:} PEM key block has an unrecognized type{:_}")
	errCantSaveKeyType     = verror.Register(pkgPath+".errCantSaveKeyType", verror.NoRetry, "{1:}{2:} key of type {3} cannot be saved{:_}")
	errCantEncryptPEMBlock = verror.Register(pkgPath+".errCantEncryptPEMBlock", verror.NoRetry, "{1:}{2:} failed to encrypt pem block{:_}")
	errCantOpenForWriting  = verror.Register(pkgPath+".errCantOpenForWriting", verror.NoRetry, "{1:}{2:} failed to open {3} for writing{:_}")
	errCantSaveKeyPair     = verror.Register(pkgPath+".errCantSaveKeyPair", verror.NoRetry, "{1:}{2:} failed to save private key to {3}, {4}{:_}")
)

// SSHKeygenNoPassphrase creates an ssh key pair using sshkeygen without
// a passphrase. It is intended for testing.
func SSHKeygenNoPassphrase(filename, comment, keyType, bits string) error {
	cmd := exec.Command("ssh-keygen",
		"-C", comment,
		"-f", filename,
		"-t", keyType,
		"-b", bits,
		"-N", "",
		"-q",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate ssh key: %v: %s", err, out)
	}
	return nil
}

func openKeyFile(keyFile string) (*os.File, error) {
	f, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, verror.New(errCantOpenForWriting, nil, keyFile, err)
	}
	return f, nil
}

// WritePEMKeyPair writes a key pair in pem format.
func WritePEMKeyPair(key interface{}, privateKeyFile, publicKeyFile string, passphrase []byte) error {
	private, err := openKeyFile(privateKeyFile)
	if err != nil {
		return err
	}
	defer private.Close()
	public, err := openKeyFile(publicKeyFile)
	if err != nil {
		return err
	}
	defer public.Close()
	if err := SavePEMKeyPair(private, public, key, passphrase); err != nil {
		return verror.New(errCantSaveKeyPair, nil, privateKeyFile, publicKeyFile, err)
	}
	return nil
}

// LoadPEMPrivateKey loads a key from 'r'. returns ErrBadPassphrase for incorrect Passphrase.
// If the key held in 'r' is unencrypted, 'passphrase' will be ignored.
func LoadPEMPrivateKey(r io.Reader, passphrase []byte) (interface{}, error) {
	pemBlockBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	pemBlock, _ := pem.Decode(pemBlockBytes)
	if pemBlock == nil {
		return nil, verror.New(errNoPEMKeyBlock, nil)
	}
	var data []byte
	if x509.IsEncryptedPEMBlock(pemBlock) {
		// Assume empty passphrase is disallowed.
		if len(passphrase) == 0 {
			return nil, verror.New(ErrPassphraseRequired, nil)
		}
		data, err = x509.DecryptPEMBlock(pemBlock, passphrase)
		if err != nil {
			return nil, verror.New(ErrBadPassphrase, nil)
		}
	} else {
		data = pemBlock.Bytes
	}

	switch pemBlock.Type {
	case ecPrivateKeyPEMType:
		key, err := x509.ParseECPrivateKey(data)
		if err != nil {
			// x509.DecryptPEMBlock may occasionally return random
			// bytes for data with a nil error when the passphrase
			// is invalid; hence, failure to parse data could be due
			// to a bad passphrase.
			return nil, verror.New(ErrBadPassphrase, nil)
		}
		return key, nil
	case pkcs8PrivateKeyPEMType:
		key, err := x509.ParsePKCS8PrivateKey(data)
		if err != nil {
			return nil, verror.New(ErrBadPassphrase, nil)
		}
		return key, nil
	}
	return nil, verror.New(errPEMKeyBlockBadType, nil, pemBlock.Type)
}

// LoadPEMPublicKey loads a public key in PEM PKIX format.
func LoadPEMPublicKey(r io.Reader) (interface{}, error) {
	pemBlockBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	pemBlock, _ := pem.Decode(pemBlockBytes)
	if pemBlock == nil {
		return nil, verror.New(errNoPEMKeyBlock, nil)
	}
	return x509.ParsePKIXPublicKey(pemBlock.Bytes)
}

// SavePEMKey marshals 'key', encrypts it using 'passphrase', and saves the bytes to 'w' in PEM format.
// If passphrase is nil, the key will not be encrypted.
//
// For example, if key is an ECDSA private key, it will be marshaled
// in ASN.1, DER format, encrypted, and then written in a PEM block.
func SavePEMKeyPair(private, public io.Writer, key interface{}, passphrase []byte) error {
	var privateData, publicData []byte
	var err error
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		if privateData, err = x509.MarshalECPrivateKey(k); err != nil {
			return err
		}
		if public != nil {
			if publicData, err = x509.MarshalPKIXPublicKey(&k.PublicKey); err != nil {
				return err
			}
		}
	case ed25519.PrivateKey:
		if privateData, err = x509.MarshalPKCS8PrivateKey(k); err != nil {
			return err
		}
		if public != nil {
			if publicData, err = x509.MarshalPKIXPublicKey(k.Public()); err != nil {
				return err
			}
		}
	default:
		return verror.New(errCantSaveKeyType, nil, fmt.Sprintf("%T", k))
	}

	var pemKey *pem.Block
	if passphrase != nil {
		pemKey, err = x509.EncryptPEMBlock(rand.Reader, ecPrivateKeyPEMType, privateData, passphrase, x509.PEMCipherAES256)
		if err != nil {
			return verror.New(errCantEncryptPEMBlock, nil, err)
		}
	} else {
		pemKey = &pem.Block{
			Type:  ecPrivateKeyPEMType,
			Bytes: privateData,
		}
	}

	if err := pem.Encode(private, pemKey); err != nil {
		return err
	}

	if public == nil {
		return nil
	}
	pemKey = &pem.Block{
		Type:  ecPublicKeyPEMType,
		Bytes: publicData,
	}
	return pem.Encode(public, pemKey)
}
