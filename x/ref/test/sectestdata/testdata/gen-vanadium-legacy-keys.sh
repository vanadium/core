#!/bin/bash


mkdir -p legacy
for algo in ecdsa256 ecdsa384 ecdsa521 ed25519; do
	go run v.io/x/ref/cmd/principal create --with-passphrase=false -key-type=$algo legacy/v23-plain-${algo}-principal
	go run v.io/x/ref/cmd/principal create --with-passphrase=true -key-type=$algo legacy/v23-encrypted-${algo}-principal <<!
password
password
!
done

rm legacy/*/dir.lock
