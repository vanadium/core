// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto"
	"crypto/ed25519"
	"embed"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/sshkeys"
	"v.io/x/ref/test/testutil/testsshagent"
)

//go:embed testdata/ssh-*
var sshKeys embed.FS

// SSHKeySetID represents a set of ssh generated keys, one set uses the key
// pairs directly, the other uses the ssh agent for signing operations and
// does not have access to the private key. Vanadium stores the ssh public
// key files in PKCS8 format internally and hence these files are provided
// for use in tests.
type SSHKeySetID int

const (
	SSHKeyPrivate SSHKeySetID = iota
	SSHKeyPublic
	SSHKeyAgentHosted
	SSHKeySetPKCS8
	SSHKeyEncrypted
)

// SSHKeydir creates a pre-populated directory of ssh keys to use in
// tests. The following keys are installed for all supported algorithms.
//	 ssh-<algo>, ssh-encrypted-<algo>, ssh-<algo>.pub, ssh-<algo>.pem,
func SSHKeydir() (string, []string, error) {
	entries, err := sshKeys.ReadDir("testdata")
	if err != nil {
		return "", nil, err
	}
	dir, err := prepopulatedDir(sshKeys, "ssh-keys", "testdata")
	if err != nil {
		return "", nil, err
	}
	names := []string{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".pub") {
			names = append(names, e.Name())
		}
	}
	return dir, names, nil
}

var sshFiles []string

func init() {
	names, err := sshKeys.ReadDir("testdata")
	if err != nil {
		panic(err)
	}
	for _, name := range names {
		sshFiles = append(sshFiles, name.Name())
	}
}

func SSHPrivateKeys() []string {
	privateKeys := []string{}
	for _, k := range sshFiles {
		if strings.HasPrefix(k, "ssh-encrypted-") {
			continue
		}
		ext := filepath.Ext(k)
		if ext == ".pem" || ext == ".pub" {
			continue
		}
		privateKeys = append(privateKeys, k)
	}
	return privateKeys
}

func sshFilename(typ keys.CryptoAlgo, set SSHKeySetID) string {
	if len(typ.String()) == 0 {
		panic(fmt.Sprintf("unrecognised key type: %v", typ))
	}
	switch set {
	case SSHKeyPrivate:
		return "ssh-" + typ.String()
	case SSHKeyEncrypted:
		return "ssh-encrypted-" + typ.String()
	case SSHKeyPublic:
		return "ssh-" + typ.String() + ".pub"
	case SSHKeySetPKCS8:
		return "ssh-" + typ.String() + ".pem"
	case SSHKeyAgentHosted:
		return "ssh-" + typ.String() + ".pub"
	}
	panic(fmt.Sprintf("unrecognised key set: %v", set))
}

func SSHPublicKeyBytes(typ keys.CryptoAlgo, set SSHKeySetID) []byte {
	if set == SSHKeyPrivate {
		panic(fmt.Sprintf("wrong key set for public keys: %v", set))
	}
	return fileContents(sshKeys, sshFilename(typ, set))
}

func SSHPrivateKey(typ keys.CryptoAlgo, set SSHKeySetID) crypto.PrivateKey {
	switch set {
	case SSHKeyPrivate:
		filename := sshFilename(typ, set)
		key, err := ssh.ParseRawPrivateKey(fileContents(sshKeys, filename))
		if err != nil {
			panic(fmt.Sprintf("failed to parse %v: %v", filename, err))
		}
		if k, ok := key.(*ed25519.PrivateKey); ok {
			return *k
		}
		return key
	case SSHKeyAgentHosted:
		pk := SSHPublicKey(typ)
		return sshkeys.NewHostedKey(pk.(ssh.PublicKey), typ.String(), nil)
	default:
		panic(fmt.Sprintf("unsupported key set %v", set))
	}
}

func SSHPublicKey(typ keys.CryptoAlgo) crypto.PublicKey {
	filename := sshFilename(typ, SSHKeyPublic)
	buf := fileContents(sshKeys, filename)
	key, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	if err != nil {
		panic(err)
	}
	return key
}

func SSHPrivateKeyBytes(typ keys.CryptoAlgo, set SSHKeySetID) []byte {
	switch set {
	case SSHKeyPrivate:
		filename := sshFilename(typ, set)
		return fileContents(sshKeys, filename)
	case SSHKeyEncrypted:
		filename := sshFilename(typ, set)
		return fileContents(sshKeys, filename)
	default:
		panic(fmt.Sprintf("unsupported key set %v", set))
	}
}

func SSHKeySigner(typ keys.CryptoAlgo, set SSHKeySetID) security.Signer {
	signer, err := signerFromCryptoKey(SSHPrivateKey(typ, set))
	if err != nil {
		panic(err)
	}
	return signer
}

func StartPreConfiguredSSHAgent() (keyDir, sockName string, cleanup func(), err error) {
	keyDir, _, err = SSHKeydir()
	if err != nil {
		return
	}
	sshTestKeys := SSHPrivateKeys()
	cleanup, sockName, err = testsshagent.StartPreconfiguredAgent(keyDir, sshTestKeys...)
	return
}
