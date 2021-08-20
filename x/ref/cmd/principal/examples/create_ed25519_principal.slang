// This examples shows how to create principal using an ed25519 key.
principalDir := expandEnv("$HOME/v23-creds")
key := createKeyPair("ed25519")
principal := useOrCreatePrincipal(key, principalDir)
printPrincipal(principal)
