rm -f ssh-*
ssh-keygen -t rsa -C rsa-2048 -b 2048 -f ssh-rsa-2048 -N ''
ssh-keygen -t ecdsa -b 256 -C ecdsa-256 -f ssh-ecdsa-256 -N ''
ssh-keygen -t ed25519 -C ed25519 -f ssh-ed25519 -N ''
