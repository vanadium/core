#!/bin/bash

export USER=anon
export HOST=host

gen_authorized_host_keys() {
	passphrase="$1"
	ssh-keygen -t rsa -C rsa-2048 -b 2048 -f ssh-rsa-2048 -N ''
	ssh-keygen -t rsa -C rsa-3072 -b 3072 -f ssh-rsa-3072 -N ''
	ssh-keygen -t rsa -C rsa-4096 -b 4096 -f ssh-rsa-4096 -N ''
	ssh-keygen -t ecdsa -b 256 -C ecdsa-256 -f ssh-ecdsa-256 -N ''
	ssh-keygen -t ecdsa -b 384 -C ecdsa-384 -f ssh-ecdsa-384 -N ''
	ssh-keygen -t ecdsa -b 521 -C ecdsa-521 -f ssh-ecdsa-521 -N ''
	ssh-keygen -t ed25519 -C ed25519 -f ssh-ed25519 -N ''
}

gen_pkcs8_keys() {
	for file in ssh-*.pub; do
		output=$(basename $file .pub).pem
		ssh-keygen -i -f $file -e -m PKCS8 | sed 's/^Comment:.*/Comment: "comment"/' > ${output}
	done
}

apply_password() {
	file=${1}
	cp ${file} tmp.key
	chmod 0600 tmp.key
	ssh-keygen -p -f tmp.key -N 'password'
	mv tmp.key $(echo ${file} | sed "s/ssh-/ssh-encrypted-/")
}

apply_passwords() {
	for file in ssh-rsa-2048 ssh-rsa-3072 ssh-rsa-4096 \
		ssh-ecdsa-256 ssh-ecdsa-384 ssh-ecdsa-521 \
		ssh-ed25519 ; do
		apply_password ${file} 
	done
}

#gen_authorized_host_keys
gen_pkcs8_keys
#apply_passwords
