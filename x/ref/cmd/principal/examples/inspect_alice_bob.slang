// This example shows how to inspect and display information about principals
// and their blessings.

alice := usePrincipal("${HOME}/creds/alice")
bob := usePrincipal("${HOME}/creds/bob")

// printPrincipal displays summary information about a principal, including
// their blessings.
printPrincipal(alice)

// publicKey obtains the public key for a principal. This is generally only
// required to create blessings for that principal.
pk := publicKey(bob)
printf("\nbob's public key: %s\n", pk)

// Print alice's blessing roots and default blessings.
// Note that there 
roots := formatBlessingRoots(alice)
defBlessings := getDefaultBlessings(alice)
str := formatBlessings(defBlessings)

printf("\nalice's blessing roots:\n%s\n", roots)
printf("\nalice's default blessings:\n%s\n", str)


bobBlessings := getDefaultBlessings(bob)
certs := getCertificateChain(bobBlessings, "bob")
printf("\nbob's cert chain\n")
printCertificateChain(certs)

fromAlice := getBlessingsForPeers(bob, "alice")
printf("\nbob's blessing from alice\n")
printBlessings(fromAlice)
