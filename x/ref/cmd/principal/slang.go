package main

import (
	"crypto"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/textutil"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/passphrase"
	"v.io/x/ref/lib/slang"
)

var keyTypeMap = map[string]seclib.KeyType{
	"ecdsa256": seclib.ECDSA256,
	"ecdsa384": seclib.ECDSA384,
	"ecdsa521": seclib.ECDSA521,
	"ed25519":  seclib.ED25519,
}

func supportedKeyTypes() []string {
	s := []string{}
	for k := range keyTypeMap {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

func defaultPrincipal(rt slang.Runtime) (security.Principal, error) {
	return v23.GetPrincipal(rt.Context()), nil
}

func printPrincipal(rt slang.Runtime, p security.Principal, printOnlyBlessingNames bool) error {
	dumpPrincipal(rt.Stdout(), p, printOnlyBlessingNames)
	return nil
}

func printBlessingsFiles(rt slang.Runtime, blessingFiles ...string) {
	for _, bf := range blessingFiles {
		dumpBlessingsFile(rt.Stdout(), bf)
	}
}

func printPublicKey(rt slang.Runtime, p security.Principal) error {
	return dumpPublicKey(rt.Stdout(), p)
}

func removePrincipal(rt slang.Runtime, dir string) error {
	return os.RemoveAll(dir)
}

func useSSHKey(rt slang.Runtime, publicKeyFile string) (crypto.PrivateKey, error) {
	return seclib.NewSSHAgentHostedKey(publicKeyFile)
}

func createKeyPair(rt slang.Runtime, keyType string) (crypto.PrivateKey, error) {
	kt, ok := keyTypeMap[strings.ToLower(keyType)]
	if !ok {
		return nil, fmt.Errorf("unsupported keytype: %v is not one of %s", keyType, strings.Join(supportedKeyTypes(), ", "))
	}
	return seclib.NewPrivateKey(kt)
}

func loadOrCreatePrincipal(rt slang.Runtime, key crypto.PrivateKey, dir string) (security.Principal, error) {
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

func defaultBlessingName(rt slang.Runtime) (string, error) {
	return createDefaultBlessingName(), nil
}

func caveats(rt slang.Runtime, expressions ...string) ([]security.Caveat, error) {
	caveatInfos := make([]caveatInfo, len(expressions))
	for i, expr := range expressions {
		exprAndParam := strings.SplitN(expr, "=", 2)
		if len(exprAndParam) != 2 {
			return nil, fmt.Errorf("incorrect caveat format: %s", expr)
		}
		caveatInfos[i] = caveatInfo{exprAndParam[0], exprAndParam[1]}
	}
	return compileCaveats(caveatInfos)
}

func parseTime(rt slang.Runtime, layout, value string) (time.Time, error) {
	return time.Parse(layout, value)
}

func deadline(rt slang.Runtime, location string, expiry time.Duration) (time.Time, error) {
	loc, err := time.LoadLocation(location)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().In(loc).Add(expiry), nil
}

func expiryCaveat(rt slang.Runtime, deadline time.Time) (security.Caveat, error) {
	c, err := security.NewExpiryCaveat(deadline)
	if err != nil {
		return security.Caveat{}, fmt.Errorf("failed to create expiration caveat: %v", err)
	}
	return c, nil
}

func expiryExpression(rt slang.Runtime, deadline time.Time) (string, error) {
	return fmt.Sprintf("\"v.io/v23/security\".ExpiryCaveat={%v, %v}", deadline.Unix(), deadline.Nanosecond()), nil
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
	slang.RegisterFunction(loadPrincipal, `returns the Principal stored in the specified directory`, "dirName")

	slang.RegisterFunction(printPrincipal, `print a Principal and associated blessing information`, "p", "printOnlyBlessingNames")
	slang.RegisterFunction(printPublicKey, `print the public key for the specified Principal`, "p")

	slang.RegisterFunction(useSSHKey, `use an ssh agent host key that corresponds to the supplied public key file`, "publicKeyFile")

	createKeyPairHelp := `create a new public/private key pair of the specified type. The suported key types are ` + strings.Join(supportedKeyTypes(), ", ") + "."

	slang.RegisterFunction(createKeyPair, createKeyPairHelp, "keyType")

	slang.RegisterFunction(loadOrCreatePrincipal, `load the existing principal if one is found in the specified directory, otherwise create a new one using the supplied key in that directory`, "privateKey", "dirName")

	slang.RegisterFunction(loadPrincipal, `load the principal stored in the specified director`, "dirName")

	slang.RegisterFunction(defaultBlessingName, `generate a blessing name based on the name on the hostname and user running this command`)

	slang.RegisterFunction(caveats, `create caveats based on the specified caveat expressions which are of the form: "package/path".CaveatName=<caveat-parameters>. For example:

	v.io/v23/security.MethodCaveat={"method"}
	`, "caveats")

	slang.RegisterFunction(deadline, "xx", "", "")
	slang.RegisterFunction(expiryCaveat, "xx", "expiry")
	slang.RegisterFunction(expiryExpression, "xx", "expiry")

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

	underline(out, "Literals")
	fmt.Fprintln(out, format(slang.Literals))

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
