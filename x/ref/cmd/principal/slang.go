package main

import (
	"errors"
	"fmt"
	"os"

	v23 "v.io/v23"
	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/passphrase"
	"v.io/x/ref/lib/security/signing/sshagent"
	"v.io/x/ref/lib/slang"
)

func defaultPrincipal(rt slang.Runtime) (security.Principal, error) {
	return v23.GetPrincipal(rt.Context()), nil
}

func printPrincipal(rt slang.Runtime, p security.Principal, short bool) error {
	dumpPrincipal(p, short)
	return nil
}

func printPublicKey(rt slang.Runtime, p security.Principal) error {
	return dumpPublicKey(p)
}

func removePrincipal(rt slang.Runtime, dir string) error {
	return os.RemoveAll(dir)
}

func createSSHPrincipal(rt slang.Runtime, publicKeyFile, dir string) (security.Principal, error) {
	privateKey := vsecurity.SSHAgentHostedKey{
		PublicKeyFile: publicKeyFile,
		Agent:         sshagent.NewClient(),
	}
	return vsecurity.CreatePersistentPrincipalUsingKey(rt.Context(), privateKey, dir, nil)
}

func createPrincipal(rt slang.Runtime, keyType, dir string) (security.Principal, error) {
	privateKey, err := seclib.NewPrivateKey(keyType)
	if err != nil {
		return nil, err
	}
	pass, err := passphrase.Get(fmt.Sprintf("Enter passphrase for %s (entering nothing will store the principal key unencrypted): ", dir))
	if err != nil {
		return nil, err
	}
	return vsecurity.CreatePersistentPrincipalUsingKey(rt.Context(), privateKey, dir, pass)
}

func loadPrincipal(rt slang.Runtime, dir string) (security.Principal, error) {
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

func init() {
	slang.RegisterFunction(defaultPrincipal, `returns the Principal that this process would use by default`)
	slang.RegisterFunction(loadPrincipal, `returns the Principal stored in the specified directory`)

	slang.RegisterFunction(printPrincipal, `print a Principal and associated blessing information`)
	slang.RegisterFunction(printPublicKey, `print the public key for the specified Principal`)

	slang.RegisterFunction(createSSHPrincipal, `create a Principal using a private key stored in an ssh agent identified by its public key file`)
	slang.RegisterFunction(createPrincipal, `creates a new Principal and private/public key pair
	using the requested algorithm/key-type. The currently supported algorithms are:
	ed25519, ecdsa521, ecdsa384 and ecdsa256. The definitive list is defined
	by v.io/x/ref/lib/security.NewPrivateKey.
`)

}
