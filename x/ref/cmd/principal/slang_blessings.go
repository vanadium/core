package main

import (
	"os"

	"v.io/v23/security"
	libsec "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/slang"
)

func defaultBlessingName(rt slang.Runtime) (string, error) {
	return createDefaultBlessingName(), nil
}

func readBlessings(rt slang.Runtime, filename string) (security.Blessings, error) {
	return decodeBlessings(filename)
}

func getCertificateChain(rt slang.Runtime, blessings security.Blessings, name string) ([]security.Certificate, error) {
	return getChainByName(blessings, name)
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

func writeBlessings(rt slang.Runtime, blessings security.Blessings, filename string) error {
	str, err := encodeBlessings(blessings)
	if err != nil {
		return err
	}
	if filename == "-" {
		dumpBlessings(rt.Stdout(), blessings)
		return nil
	}
	return os.WriteFile(filename, []byte(str), 0600)
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

func init() {
	slang.RegisterFunction(readBlessings, `Read blessings a from a file (or stdin if filename is '-').`, "filename")

	slang.RegisterFunction(writeBlessings, `Write blessings to a file (or stdout if filename is '-').`, "blessings", "filename")

	slang.RegisterFunction(defaultBlessingName, `Generate a blessing name based on the name on the hostname and user running this command.`)

	slang.RegisterFunction(createBlessings, `Create blessings signed by the supplied principal.`, "principal", "name", "caveats")

	slang.RegisterFunction(setDefaultBlessings, `Set the default blessings for the specified principal, mark them as shareable with all peers and also add it the blessing's root
	to the principal's BlessingRoot's. Note, that the public key of the blessings must
	match the public key of the principal, that is, the blessings were signed by this
	same principal.

	It is an error to call SetDefault with a blessings whose public key does not match the PublicKey of the principal for which this store hosts blessings.
	`, "principal", "blessings")

	slang.RegisterFunction(getDefaultBlessings, `Return the default blessings for the specified principal.`, "principal")

	slang.RegisterFunction(blessingPattern, `Create a blessing pattern, ie. a pattern to be matched by a specific blessing.
	
	A pattern can either be a blessing (slash-separated human-readable string) or a blessing ending in "/$". A pattern ending in "/$" is matched exactly by the blessing specified by the pattern string with the "/$" suffix stripped out. For example, the pattern "a/b/c/$" is matched by exactly by the blessing "a/b/c".

	A pattern not ending in "/$" is more permissive, and is also matched by blessings that are extensions of the pattern (including the pattern itself). For example, the pattern "a/b/c" is matched by the blessings "a/b/c", "a/b/c/x", "a/b/c/x/y", etc.`, "pattern")

	slang.RegisterFunction(setBlessingsForPeers, `Set the blessings to be shared with peers.
	
	Set(b, pattern) marks the intention to reveal b to peers who present blessings of their own matching pattern.

	If multiple calls to Set are made with the same pattern, the last call prevails.

	clearBlessingsForPeers can be used to remove the blessings previously associated with the pattern.

	It is an error to call Set with "blessings" whose public key does not match the PublicKey of the principal for which this store hosts blessings.

	Set returns the Blessings object which was previously associated with the pattern.
	
	`, "blessings", "pattern", "forPeers")

	slang.RegisterFunction(clearBlessingsForPeers, `Clear any blessings associated with the specified pattern`, "principal", "pattern")

	slang.RegisterFunction(blessPrincipal, `Bless another principal with the supplied blessings and caveats. If no caveats are provided a caveat that allows all access is used; this assumes that the supplied blessings are appropriately restrictive.`, "blessor", "blessed", "blessings", "extension", "caveats")

	slang.RegisterFunction(getCertificateChain, `Return the certificate identified by the supplied name.`, "blessings", "name")

	slang.RegisterFunction(getCaveats, `Return the caveats in the supplied certificate chain`, "certificates")

	slang.RegisterFunction(getBlessingsForPeers, `Return the blessings that are bound to the specified peers`, "principal", "peers")

}
