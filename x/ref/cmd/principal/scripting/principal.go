package scripting

import (
	"crypto"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	v23 "v.io/v23"
	"v.io/v23/security"
	"v.io/x/lib/textutil"
	"v.io/x/ref/cmd/principal/internal"
	seclib "v.io/x/ref/lib/security"
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

func createKeyPair(rt slang.Runtime, keyType string) (crypto.PrivateKey, error) {
	kt, ok := internal.IsSupportedKeyType(keyType)
	if !ok {
		return nil, fmt.Errorf("unsupported keytype: %v is not one of %s", keyType, strings.Join(internal.SupportedKeyTypes(), ", "))
	}
	return seclib.NewPrivateKey(kt)
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

func addToRoots(rt slang.Runtime, p security.Principal, blessings security.Blessings) error {
	return security.AddToRoots(p, blessings)
}

func publicKey(rt slang.Runtime, p security.Principal) (security.PublicKey, error) {
	return p.PublicKey(), nil
}

func init() {
	slang.RegisterFunction(defaultPrincipal, "principal", `Returns the Principal that this process would use by default.`)

	slang.RegisterFunction(useSSHKey, "principal", `Use an ssh agent host key that corresponds to the supplied public key file. Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "publicKeyFile")

	createKeyPairHelp := `Create a new public/private key pair of the specified type. The suported key types are ` + strings.Join(internal.SupportedKeyTypes(), ", ") + "."

	slang.RegisterFunction(createKeyPair, "principal", createKeyPairHelp, "keyType")

	slang.RegisterFunction(useOrCreatePrincipal, "principal", `Use the existing principal if one is found in the specified directory, otherwise create a new one using the supplied key in that directory. Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "privateKey", "dirName")

	slang.RegisterFunction(usePrincipal, "principal", `Use the principal stored in the specified directory.  Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "dirName")

	slang.RegisterFunction(publicKey, "principal", `Return the public key for the specified principal`, "principal")

	slang.RegisterFunction(removePrincipal, "principal", `Remove the specified principal directory. Note, that shell variable expansion is performed on the supplied dirname, hence $HOME/dir works as expected.`, "dirname")

	slang.RegisterFunction(addToRoots, `addToRoots marks the root principals of all blessing chains represented by 'blessings' as an authority on blessing chains beginning at that root name in p.BlessingRoots().
	
	For example, if blessings represents the blessing chains ["alice:friend:spouse", "charlie:family:daughter"] then AddToRoots(blessing) will mark the root public key of the chain "alice:friend:bob" as the authority on all blessings that match the pattern "alice", and root public
	key of the chain "charlie:family:daughter" as an authority on all blessings that match the pattern "charlie".`, "principal", "principal", "blessings")

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
