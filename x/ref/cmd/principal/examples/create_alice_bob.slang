// This example shows to create or use the principals alice and bob and
// to set their default blessings.
//
// The 'useOrCreate' functions are used so as to ensure that after they
// are run the principal exists, even if a key is a created and not used.
// Key creation is separate from the useOrCreatePrincipal function to allow for
// different types of key with cluttering the API for useOrCreatePrincipal
// with all of the possible options.
key := createKeyPair("ed25519")
alice := useOrCreatePrincipal(key, "${HOME}/creds/alice")
key = createKeyPair("ed25519")
bob := useOrCreatePrincipal(key, "${HOME}/creds/bob")

blessing := createBlessings(alice, "alice")
setDefaultBlessings(alice, blessing)

blessing  = createBlessings(bob, "bob")
setDefaultBlessings(bob, blessing)

printf("Principal: alice\n")
printPrincipal(alice)

printf("\nPrincipal: bob\n")
printPrincipal(bob)
