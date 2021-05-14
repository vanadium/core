// This example shows how to create a blessing granted to bob by alice.
bob := usePrincipal("${HOME}/creds/bob")
alice := usePrincipal("${HOME}/creds/alice")

// Create a blessing whereby alice 'blesses' bob with the caveat that the
// blessing is valid for 24h from now.
expiresIn24h := expiryCaveat("24h")
bobPublicKey := publicKey(bob)
rootBlessing := getDefaultBlessings(alice)
blessings := blessPrincipal(alice, bobPublicKey, rootBlessing, "friend:bob", expiresIn24h)

forAlice := blessingPattern("alice")
setBlessingsForPeers(bob, blessings, forAlice)


