package main

import (
	"fmt"

	"v.io/v23/security"
	"v.io/x/ref/lib/slang"
)

func printPrincipal(rt slang.Runtime, p security.Principal) error {
	dumpPrincipal(rt.Stdout(), p, false)
	return nil
}
func printPublicKey(rt slang.Runtime, p security.Principal) error {
	return dumpPublicKey(rt.Stdout(), p)
}

func printBlessings(rt slang.Runtime, blessings security.Blessings) error {
	dumpBlessings(rt.Stdout(), blessings)
	return nil
}

func printCertificates(rt slang.Runtime, certs []security.Certificate) error {
	for i, cert := range certs {
		key, err := security.UnmarshalPublicKey(cert.PublicKey)
		if err != nil {
			return fmt.Errorf("<invalid PublicKey: %v>", err)
		}
		fmt.Fprintf(rt.Stdout(), "%v: %v\n", i, key)
	}
	return nil
}

func printBlessingRoots(rt slang.Runtime, p security.Principal) error {
	fmt.Fprintf(rt.Stdout(), "%v", p.Roots().DebugString())
	return nil
}

func printCaveats(rt slang.Runtime, caveats ...security.Caveat) error {
	str, err := formatCaveats(caveats)
	if err != nil {
		return err
	}
	for _, s := range str {
		fmt.Fprintln(rt.Stdout(), s)
	}
	return nil
}

func init() {
	slang.RegisterFunction(printPrincipal, "print", `Print a Principal and associated blessing information.`, "principal")
	slang.RegisterFunction(printPublicKey, "print", `Print the public key for the specified principal.`, "principal")
	slang.RegisterFunction(readBlessings, "print", `Read blessings from the spefified file.`, "filename")
	slang.RegisterFunction(printBlessings, "print", `Print the supplied blessings.`, "blessings")
	slang.RegisterFunction(printCaveats, "print", `Print the supplied caveats.`, "caveats")
	slang.RegisterFunction(printCertificates, "print", `Print the public key of each certificate in the specified chain`, "certificates")
	slang.RegisterFunction(printBlessingRoots, "print", `Print the blessing roots for the specified principal.`, "principal")
}
