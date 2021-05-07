package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/textutil"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/passphrase"
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

func createSSHKeyPairing(rt slang.Runtime, publicKeyFile string) (seclib.KeyPair, error) {
	return seclib.NewSSHAgentHostedKeyPair(publicKeyFile)
}

func createKeyPair(rt slang.Runtime, keyType string) (seclib.KeyPair, error) {
	return seclib.NewPrivateKey(keyType)
}

func loadOrCreatePrincipal(rt slang.Runtime, key seclib.KeyPair, dir string) (security.Principal, error) {
	p, err := loadPrincipal(rt, dir)
	if err == nil {
		return p, err
	}
	pass, err := passphrase.Get(fmt.Sprintf("Enter passphrase for %s (entering nothing will store the principal key unencrypted): ", dir))
	if err != nil {
		return nil, err
	}
	return seclib.CreatePersistentPrincipalUsingKey(rt.Context(), key, dir, pass)
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

func runScript(ctx *context.T, rd io.Reader) error {
	buf, err := io.ReadAll(rd)
	if err != nil {
		return err
	}
	scr := &slang.Script{}
	return scr.ExecuteBytes(ctx, buf)
}

func runScriptFile(ctx *context.T, name string) error {
	if name == "-" || len(name) == 0 {
		return runScript(ctx, os.Stdin)
	}
	rd, err := os.Open(name)
	if err != nil {
		return err
	}
	return runScript(ctx, rd)

}

func init() {
	slang.RegisterFunction(defaultPrincipal, `returns the Principal that this process would use by default`)
	slang.RegisterFunction(loadPrincipal, `returns the Principal stored in the specified directory`, "p", "shortFormat")

	slang.RegisterFunction(printPrincipal, `print a Principal and associated blessing information`, "p")
	slang.RegisterFunction(printPublicKey, `print the public key for the specified Principal`, "p")

	slang.RegisterFunction(createKeyPair, `create a Principal using a private key stored in an ssh agent identified by its public key file`)
	slang.RegisterFunction(createPrincipal, `creates a new Principal and private/public key pair using the requested algorithm/key-type. The currently supported algorithms are: ed25519, ecdsa521, ecdsa384 and ecdsa256. The definitive list is defined by v.io/x/ref/lib/security.NewPrivateKey.
`)

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

func scriptDocumentation() string {
	out := &strings.Builder{}

	underline(out, "Summary")

	fmt.Fprintln(out, format(slang.Summary), "")

	underline(out, "Examples")
	fmt.Fprintln(out, slang.Examples)

	underline(out, "Available Functions")
	for _, fn := range slang.RegisteredFunctions() {
		fmt.Fprintf(out, "%s\n", fn.Function)
		fmt.Fprintf(out, format(fn.Help, "  "))
		fmt.Fprintln(out)
	}
	return out.String()
}
