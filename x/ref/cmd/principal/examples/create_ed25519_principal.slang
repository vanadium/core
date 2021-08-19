// This examples shows how to create principal using an ed25519 key.
principalDir := expandEnv("$HOME/v23-creds")
key := createKeyPair("ed25519")
principal := loadOrCreatePrincipal(key, principalDir)
printPrincipal(principal, false)
