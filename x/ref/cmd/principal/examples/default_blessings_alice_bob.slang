// This example shows how to set default blessings for an
// existing principal.
alice := usePrincipal("./alice")
bob := usePrincipal("./bob")

// Set default blessings for alice.
blessing := createBlessings(alice, "alice")
setDefaultBlessings(alice, blessing)

// Set default blessings for bob.
blessing  = createBlessings(bob, "bob")
setDefaultBlessings(bob, blessing)

printf("Principal: alice\n")
printPrincipal(alice)

printf("\nPrincipal: bob\n")
printPrincipal(bob)
