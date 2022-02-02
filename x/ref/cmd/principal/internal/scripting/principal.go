// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scripting

import (
	"crypto"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	v23 "v.io/v23"
	"v.io/v23/security"
	"v.io/x/lib/textutil"
	"v.io/x/ref/cmd/principal/internal"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/passphrase"
	"v.io/x/ref/lib/slang"
)

func defaultPrincipal(rt slang.Runtime) (security.Principal, error) {
	return v23.GetPrincipal(rt.Context()), nil
}

func removePrincipal(rt slang.Runtime, dir string) error {
	dir = os.ExpandEnv(dir)
	return os.RemoveAll(dir)
}

func useSSHKey(rt slang.Runtime, publicKeyFile string) (crypto.PrivateKey, error) {
	publicKeyFile = os.ExpandEnv(publicKeyFile)
	return seclib.NewSSHAgentHostedKey(publicKeyFile)
}

func useSSLKey(rt slang.Runtime, sslKeyFile string) (crypto.PrivateKey, error) {
	sslKeyFile = os.ExpandEnv(sslKeyFile)
	keyBytes, err := os.ReadFile(sslKeyFile)
	if err != nil {
		return nil, err
	}
	var privateKey crypto.PrivateKey
	var pass []byte
	for {
		key, err := seclib.KeyRegistrar().ParsePrivateKey(rt.Context(), keyBytes, pass)
		if err == nil {
			privateKey = key
			break
		}
		if err == seclib.ErrPassphraseRequired || err == seclib.ErrBadPassphrase {
			pass, _ = passphrase.Get(fmt.Sprintf("Enter passphrase for %s: ", sslKeyFile))
			continue
		}
		return nil, err
	}
	return privateKey, nil
}

func createKeyPair(rt slang.Runtime, keyType string) (crypto.PrivateKey, error) {
	kt, ok := internal.IsSupportedKeyType(keyType)
	if !ok {
		return nil, fmt.Errorf("unsupported keytype: %v is not one of %s", keyType, strings.Join(internal.SupportedKeyTypes(), ", "))
	}
	return keys.NewPrivateKeyForAlgo(kt)
}

func useOrCreatePrincipal(rt slang.Runtime, key crypto.PrivateKey, dir string) (security.Principal, error) {
	p, err := usePrincipal(rt, dir)
	if err == nil {
		return p, err
	}
	dir = os.ExpandEnv(dir)
	pass, err := passphrase.Get(fmt.Sprintf("Enter passphrase for %s (entering nothing will store the principal key unencrypted): ", dir))
	if err != nil {
		return nil, err
	}
	return seclib.CreatePersistentPrincipalUsingKey(rt.Context(), key, dir, pass)
}

func usePrincipal(rt slang.Runtime, dir string) (security.Principal, error) {
	dir = os.ExpandEnv(dir)
	var pass []byte
	for {
		p, err := seclib.LoadPersistentPrincipal(dir, pass)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, seclib.ErrBadPassphrase) {
			return nil, err
		}
		pass, err = passphrase.Get(fmt.Sprintf("Enter passphrase for %s (entering nothing will store the principal key unencrypted): ", dir))
		if err != nil {
			return nil, err
		}
	}
}

func usePublicKey(rt slang.Runtime, dir string) (security.PublicKey, error) {
	p, err := usePrincipal(rt, dir)
	if err != nil {
		return nil, err
	}
	return p.PublicKey(), nil
}

func addToRoots(rt slang.Runtime, p security.Principal, blessings security.Blessings) error {
	return security.AddToRoots(p, blessings)
}

func publicKey(rt slang.Runtime, p security.Principal) (security.PublicKey, error) {
	return p.PublicKey(), nil
}

func sshPublicKeyFromFile(filename string) (ssh.PublicKey, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	key, err := seclib.KeyRegistrar().ParsePublicKey(data)
	if err != nil {
		return nil, err
	}
	if sshkey, ok := key.(ssh.PublicKey); ok {
		return sshkey, nil
	}
	return nil, fmt.Errorf("%v does not contain an ssh public key: has %T", filename, key)
}

func sshPublicKeyMD5(rt slang.Runtime, filename string) (string, error) {
	key, err := sshPublicKeyFromFile(filename)
	if err != nil {
		return "", err
	}
	return ssh.FingerprintLegacyMD5(key), nil
}

func sshPublicKeySHA256(rt slang.Runtime, filename string) (string, error) {
	key, err := sshPublicKeyFromFile(filename)
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(key), nil
}

func init() {
	slang.RegisterFunction(defaultPrincipal, "principal", `Returns the Principal that this process would use by default.`)

	slang.RegisterFunction(useSSHKey, "principal", `Use an ssh agent host key that corresponds to the supplied public key file. Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "publicKeyFile")

	createKeyPairHelp := `Create a new public/private key pair of the specified type. The suported key types are ` + strings.Join(internal.SupportedKeyTypes(), ", ") + "."

	slang.RegisterFunction(createKeyPair, "principal", createKeyPairHelp, "keyType")

	slang.RegisterFunction(useOrCreatePrincipal, "principal", `Use the existing principal if one is found in the specified directory, otherwise create a new one using the supplied key in that directory. Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "privateKey", "dirName")

	slang.RegisterFunction(usePrincipal, "principal", `Use the principal stored in the specified directory.  Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "dirName")

	slang.RegisterFunction(usePublicKey, "principal", `Use the public key of the principal stored in the specified directory. Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "dirName")

	slang.RegisterFunction(useSSLKey, "principal", `Use the private/public key of the principal specified SSL/TLS key file. Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "dirName")

	slang.RegisterFunction(publicKey, "principal", `Return the public key for the specified principal`, "principal")

	slang.RegisterFunction(sshPublicKeyMD5, "principal", `Return the md5 signature of the openssh key in the specified file as would be displayed by sshkey-gen -l -m md5`, "filename")

	slang.RegisterFunction(sshPublicKeySHA256, "principal", `Return the sha256 signature of the openssh key in the specified file as would be displayed by sshkey-gen -l -m sha256`, "filename")

	slang.RegisterFunction(removePrincipal, "principal", `Remove the specified principal directory. Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "dirname")

	slang.RegisterFunction(addToRoots, "principal", `addToRoots marks the root principals of all blessing chains represented by 'blessings' as an authority on blessing chains beginning at that root name in p.BlessingRoots().
	
	For example, if blessings represents the blessing chains ["alice:friend:spouse", "charlie:family:daughter"] then AddToRoots(blessing) will mark the root public key of the chain "alice:friend:bob" as the authority on all blessings that match the pattern "alice", and root public key of the chain "charlie:family:daughter" as an authority on all blessings that match the pattern "charlie".`, "principal", "blessings")
}

func underline(out io.Writer, msg string) {
	fmt.Fprintf(out, "%s\n%s\n\n", msg, strings.Repeat("=", len(msg)))
}

func format(msg string, indents ...string) string {
	out := &strings.Builder{}
	wr := textutil.NewUTF8WrapWriter(out, 70)
	wr.SetIndents(indents...)
	wr.Write([]byte(msg))
	wr.Flush()
	return out.String()
}
