// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scripting

import (
	"fmt"
	"strings"

	"v.io/v23/security"
	"v.io/x/ref/cmd/principal/internal"
	"v.io/x/ref/lib/slang"
)

func formatPrincipal(rt slang.Runtime, p security.Principal) (string, error) {
	out := &strings.Builder{}
	internal.DumpPrincipal(out, p, false)
	return out.String(), nil
}

func printPrincipal(rt slang.Runtime, p security.Principal) error {
	internal.DumpPrincipal(rt.Stdout(), p, false)
	return nil
}

func formatPublicKey(rt slang.Runtime, p security.Principal) (string, error) {
	out := &strings.Builder{}
	internal.DumpPublicKey(out, p, true)
	return out.String(), nil
}

func printPublicKey(rt slang.Runtime, p security.Principal) error {
	internal.DumpPublicKey(rt.Stdout(), p, true)
	return nil
}

func formatBlessings(rt slang.Runtime, blessings security.Blessings) (string, error) {
	out := &strings.Builder{}
	internal.DumpBlessings(out, blessings)
	return out.String(), nil
}

func printBlessings(rt slang.Runtime, blessings security.Blessings) error {
	internal.DumpBlessings(rt.Stdout(), blessings)
	return nil
}

func formatCertificateChain(rt slang.Runtime, chain []security.Certificate) (string, error) {
	out := &strings.Builder{}
	internal.DumpCertificateChain(out, chain)
	return out.String(), nil
}

func printCertificateChain(rt slang.Runtime, chain []security.Certificate) error {
	internal.DumpCertificateChain(rt.Stdout(), chain)
	return nil
}

func formatBlessingRoots(rt slang.Runtime, p security.Principal) (string, error) {
	return p.Roots().DebugString(), nil
}

func printBlessingRoots(rt slang.Runtime, p security.Principal) error {
	fmt.Fprintf(rt.Stdout(), "%v", p.Roots().DebugString())
	return nil
}

func formatCaveats(rt slang.Runtime, caveats ...security.Caveat) (string, error) {
	lines, err := internal.FormatCaveats(caveats)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func printCaveats(rt slang.Runtime, caveats ...security.Caveat) error {
	lines, err := formatCaveats(rt, caveats...)
	if err != nil {
		return err
	}
	fmt.Fprintln(rt.Stdout(), lines)
	return nil
}

func formatCaveatsInChain(rt slang.Runtime, chain []security.Certificate) (string, error) {
	lines, err := internal.FormatCaveatsInChain(chain)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func printCaveatsInChain(rt slang.Runtime, chain []security.Certificate) error {
	lines, err := formatCaveatsInChain(rt, chain)
	if err != nil {
		return err
	}
	fmt.Fprintln(rt.Stdout(), lines)
	return nil
}

func init() {
	slang.RegisterFunction(printPrincipal, "print", `Print a Principal and associated blessing information.`, "principal")
	slang.RegisterFunction(printPublicKey, "print", `Print the public key for the specified principal.`, "principal")
	slang.RegisterFunction(printBlessings, "print", `Print the supplied blessings.`, "blessings")
	slang.RegisterFunction(printCertificateChain, "print", `Print certificate chain, including the public key of each cert in the chain`, "certificates")
	slang.RegisterFunction(printBlessingRoots, "print", `Print the blessing roots for the specified principal.`, "principal")
	slang.RegisterFunction(printCaveats, "print", `Print the supplied caveats.`, "caveats")
	slang.RegisterFunction(printCaveatsInChain, "print", `Print the caveats inthe specified certificate chain.`, "chain")

	slang.RegisterFunction(formatPrincipal, "print", `Print a Principal and associated blessing information.`, "principal")
	slang.RegisterFunction(formatPublicKey, "print", `Print the public key for the specified principal.`, "principal")
	slang.RegisterFunction(formatBlessings, "print", `Print the supplied blessings.`, "blessings")
	slang.RegisterFunction(formatCertificateChain, "print", `Print certificate chain, including the public key of each cert in the chain`, "certificates")
	slang.RegisterFunction(formatBlessingRoots, "print", `Print the blessing roots for the specified principal.`, "principal")
	slang.RegisterFunction(formatCaveats, "print", `Print the supplied caveats.`, "caveats")
	slang.RegisterFunction(formatCaveatsInChain, "print", `Print the caveats inthe specified certificate chain.`, "chain")
}
