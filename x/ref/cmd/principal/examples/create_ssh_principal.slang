// This examples shows how to use an ssh-agent managed
// private key rather than a locally created one.
sshKey := useSSHKey("$HOME/.ssh/cos-scg.pub")
principal := useOrCreatePrincipal(sshKey, "${HOME}/creds/ssh-principal")
printPrincipal(principal)
