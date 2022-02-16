#!/bin/bash

# need bash version 4.0 and later

declare -A algomap
algomap[ecdsa256]="ecdsa-256"
algomap[ecdsa384]="ecdsa-384"
algomap[ecdsa521]="ecdsa-521"
algomap[ed25519]="ed25519"
algomap[rsa2048]="rsa-2048"
algomap[rsa4096]="rsa-4096"

mkdir -p legacy
for algo in ecdsa256 ecdsa384 ecdsa521 ed25519 rsa2048 rsa4096; do
	an=${algomap[$algo]}
	go run v.io/x/ref/cmd/principal create --with-passphrase=false -key-type=$algo legacy/v23-plain-${an}-principal
	go run v.io/x/ref/cmd/principal create --with-passphrase=true -key-type=$algo legacy/v23-encrypted-${an}-principal <<!
password
password
!
done

rm legacy/*/dir.lock
