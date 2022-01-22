// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto"
	"embed"
	_ "embed"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/x/ref/test/testutil/testsshagent"
)

//go:embed testdata/ssh-*
var sshKeys embed.FS

// SSHKeySetID represents a set of ssh generated keys, one set uses the key
// pairs directly, the other uses the ssh agent for signing operations and
// does not have access to the private key. Vanadium stores the ssh public
// key files in ssh2 RFC4716 internally and hence these files are provided
// for use in tests.
type SSHKeySetID int

const (
	SSHkeySetNative SSHKeySetID = iota
	SSHKeyAgentHosted
	SSHKeySetRFC4716
)

// SSHKeydir creates a pre-populated directory of ssh keys to use in
// tests. The following keys are installed:
//	 ssh-ecdsa-256, ssh-ecdsa-256.pub, ssh-ecdsa-256.pem
//	 ssh-ecdsa-384, ssh-ecdsa-384.pub, ssh-ecdsa-384.pem
//	 ssh-ecdsa-521, ssh-ecdsa-521.pub, ssh-ecdsa-521.pem
//	 ssh-ed25519, ssh-ed25519.pub, ssh-ed25519.pem
//	 ssh-rsa-2048, ssh-rsa-2048.pub, ssh-rsa-2048.pem
//	 ssh-rsa-3072, ssh-rsa-3072.pub, ssh-rsa-3072.pem
//	 ssh-rsa-4096, ssh-rsa-4096.pub, ssh-rsa-4096.pem
func SSHKeydir() (string, []string, error) {
	entries, err := sshKeys.ReadDir("testdata")
	if err != nil {
		return "", nil, err
	}
	dir, err := prepopulatedDir("ssh-keys", "testdata", sshKeys)
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
		ext := filepath.Ext(k)
		if ext == ".pem" || ext == ".pub" {
			continue
		}
		privateKeys = append(privateKeys, k)
	}
	return privateKeys
}

func sshFilename(typ KeyType, set SSHKeySetID) string {
	if len(typ.String()) == 0 {
		panic(fmt.Sprintf("unrecognised key type: %v", typ))
	}
	switch set {
	case SSHkeySetNative:
		return "ssh-" + typ.String()
	case SSHKeyAgentHosted:
		return "ssh-" + typ.String() + ".pub"
	case SSHKeySetRFC4716:
		return "ssh-" + typ.String() + ".pem"
	}
	panic(fmt.Sprintf("unrecognised key set: %v", set))
}

func SSHPublicKey(typ KeyType, set SSHKeySetID) []byte {
	if set != SSHKeyAgentHosted && set != SSHKeySetRFC4716 {
		panic(fmt.Sprintf("wrong key set for public keys: %v", set))
	}
	return sshFileContents(sshKeys, sshFilename(typ, set))
}

func sshFileContents(fs embed.FS, filename string) []byte {
	filename = path.Join("testdata", filename)
	data, err := fs.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return data
}

func SSHPrivateKey(typ KeyType, set SSHKeySetID) crypto.PrivateKey {
	switch set {
	case SSHkeySetNative:
		filename := sshFilename(typ, set)
		key, err := ssh.ParseRawPrivateKey(sshFileContents(sshKeys, filename))
		if err != nil {
			panic(fmt.Sprintf("failed to parse %v: %v", filename, err))
		}
		return key
	case SSHKeyAgentHosted:
		// TODO(cnicolaou): implement this once the ssh public key handling
		//                  is cleaned up.
		return nil
	default:
		panic(fmt.Sprintf("unsupported key set %v", set))
	}
}

func SSHKeySigner(typ KeyType, set SSHKeySetID) security.Signer {
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
