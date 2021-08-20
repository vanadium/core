// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scripting

import (
	"v.io/v23/security"
	"v.io/x/ref/cmd/principal/internal"
	libsec "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/slang"
)

func defaultBlessingName(rt slang.Runtime) (string, error) {
	return internal.CreateDefaultBlessingName(), nil
}

func readBlessings(rt slang.Runtime, filename string) (security.Blessings, error) {
	return internal.DecodeBlessingsFile(filename)
}

func writeBlessings(rt slang.Runtime, filename string, blessings security.Blessings) error {
	return internal.EncodeBlessingsFile(filename, rt.Stdout(), blessings)
}

func writeBlessingRoots(rt slang.Runtime, filename string, blessings security.Blessings) error {
	return internal.EncodeBlessingRootsFile(filename, rt.Stdout(), blessings)
}

func getCertificateChain(rt slang.Runtime, blessings security.Blessings, name string) ([]security.Certificate, error) {
	return internal.GetChainByName(blessings, name)
}

func getCaveats(rt slang.Runtime, chain []security.Certificate) ([]security.Caveat, error) {
	var cavs []security.Caveat
	for _, cert := range chain {
		cavs = append(cavs, cert.Caveats...)
	}
	return cavs, nil
}

func getBlessingsForPeers(rt slang.Runtime, p security.Principal, peers ...string) (security.Blessings, error) {
	return p.BlessingStore().ForPeer(peers...), nil
}

func createBlessings(rt slang.Runtime, p security.Principal, name string, caveats ...security.Caveat) (security.Blessings, error) {
	return p.BlessSelf(name, caveats...)
}

func setDefaultBlessings(rt slang.Runtime, p security.Principal, blessings security.Blessings) error {
	return libsec.SetDefaultBlessings(p, blessings)
}

func getDefaultBlessings(rt slang.Runtime, p security.Principal) (security.Blessings, error) {
	b, _ := p.BlessingStore().Default()
	return b, nil
}

func blessingPattern(rt slang.Runtime, pattern string) (security.BlessingPattern, error) {
	return security.BlessingPattern(pattern), nil
}

func allPrincipalsBlessingPattern(rt slang.Runtime) (security.BlessingPattern, error) {
	return security.AllPrincipals, nil
}

func setBlessingsForPeers(rt slang.Runtime, p security.Principal, blessings security.Blessings, forPeers security.BlessingPattern) error {
	_, err := p.BlessingStore().Set(blessings, forPeers)
	return err
}

func clearBlessingsForPeers(rt slang.Runtime, p security.Principal, forPeers security.BlessingPattern) error {
	_, err := p.BlessingStore().Set(security.Blessings{}, forPeers)
	return err
}

func blessPrincipal(rt slang.Runtime, blessor security.Principal, blessed security.PublicKey,
	blessings security.Blessings, extension string, caveats ...security.Caveat) (security.Blessings, error) {
	switch len(caveats) {
	case 0:
		return blessor.Bless(blessed, blessings, extension, security.UnconstrainedUse())
	case 1:
		return blessor.Bless(blessed, blessings, extension, caveats[0])
	default:
		return blessor.Bless(blessed, blessings, extension, caveats[0], caveats[1:]...)
	}
}

func unionBlessings(rt slang.Runtime, blessings ...security.Blessings) (security.Blessings, error) {
	return security.UnionOfBlessings(blessings...)
}

func encodeBlessingsBase64(rt slang.Runtime, blessings ...security.Blessings) (string, error) {
	u, err := security.UnionOfBlessings(blessings...)
	if err != nil {
		return "", err
	}
	return libsec.EncodeBlessingsBase64(u)
}

func decodeBlessingsBase64(rt slang.Runtime, encoded string) (security.Blessings, error) {
	return libsec.DecodeBlessingsBase64(encoded)
}

func init() {
	slang.RegisterFunction(readBlessings, "blessings", `Read blessings a from a file (or stdin if filename is '-').`, "filename")

	slang.RegisterFunction(writeBlessings, "blessings", `Write blessings to a file (or stdout if filename is '-').`, "filename", "blessings")

	slang.RegisterFunction(defaultBlessingName, "blessings", `Generate a blessing name based on the name on the hostname and user running this command.`)

	slang.RegisterFunction(createBlessings, "blessings", `Create blessings signed by the supplied principal.`, "principal", "name", "caveats")

	slang.RegisterFunction(setDefaultBlessings, "blessings", `Set the default blessings for the specified principal, mark them as shareable with all peers and also add it the blessing's root
	to the principal's BlessingRoot's. Note, that the public key of the blessings must
	match the public key of the principal, that is, the blessings were signed by this
	same principal.

	It is an error to call SetDefault with a blessings whose public key does not match the PublicKey of the principal for which this store hosts blessings.
	`, "principal", "blessings")

	slang.RegisterFunction(getDefaultBlessings, "blessings", `Return the default blessings for the specified principal.`, "principal")

	slang.RegisterFunction(blessingPattern, "blessings", `Create a blessing pattern, ie. a pattern to be matched by a specific blessing.
	
	A pattern can either be a blessing (slash-separated human-readable string) or a blessing ending in "/$". A pattern ending in "/$" is matched exactly by the blessing specified by the pattern string with the "/$" suffix stripped out. For example, the pattern "a/b/c/$" is matched by exactly by the blessing "a/b/c".

	A pattern not ending in "/$" is more permissive, and is also matched by blessings that are extensions of the pattern (including the pattern itself). For example, the pattern "a/b/c" is matched by the blessings "a/b/c", "a/b/c/x", "a/b/c/x/y", etc.`, "pattern")

	slang.RegisterFunction(allPrincipalsBlessingPattern, "blessings", `Returns the blessing pattern that matches all principals, ie. "..."`)

	slang.RegisterFunction(setBlessingsForPeers, "blessings", `Set the blessings to be shared with peers.
	
	Set(b, pattern) marks the intention to reveal b to peers who present blessings of their own matching pattern.

	If multiple calls to Set are made with the same pattern, the last call prevails.

	clearBlessingsForPeers can be used to remove the blessings previously associated with the pattern.

	It is an error to call Set with "blessings" whose public key does not match the PublicKey of the principal for which this store hosts blessings.

	Set returns the Blessings object which was previously associated with the pattern.
	
	`, "blessings", "pattern", "forPeers")

	slang.RegisterFunction(clearBlessingsForPeers, "blessings", `Clear any blessings associated with the specified pattern`, "principal", "pattern")

	slang.RegisterFunction(blessPrincipal, "blessings", `Bless another principal with the supplied blessings and caveats. If no caveats are provided a caveat that allows all access is used; this assumes that the supplied blessings are appropriately restrictive.`, "blessor", "blessed", "blessings", "extension", "caveats")

	slang.RegisterFunction(getCertificateChain, "blessings", `Return the certificate identified by the supplied name.`, "blessings", "name")

	slang.RegisterFunction(getCaveats, "blessings", `Return the caveats in the supplied certificate chain`, "certificates")

	slang.RegisterFunction(getBlessingsForPeers, "blessings", `Return the blessings that are bound to the specified peers`, "principal", "peers")

	slang.RegisterFunction(writeBlessingRoots, "blessings", `Write out the blessings of the identity providers of the supplied blessings to the specified file or stdout if '-'.  One
	line per identity provider, each line is a base64url-encoded (RFC 4648, Section
	5) vom-encoded Blessings object.`, "filename", "blessings")

	slang.RegisterFunction(unionBlessings, "blessings", `Return the union of the supplied blessings.`, "blessings")

	slang.RegisterFunction(encodeBlessingsBase64, "blessings", `Encode blessings to a base64 url encoded string.`, "blessings")

	slang.RegisterFunction(decodeBlessingsBase64, "blessings", `Decode blessings from a base64 url encoded string.`, "encoded")
}
