
set -e

keyfile=vanadium.io.key.pem
cafile=vanadium.io.ca.pem

genca() {
	openssl genrsa 2048 > ${keyfile}
	openssl req -x509 -new -nodes -key ${keyfile} -sha256 -days 1825 -subj '/CN=vanadium.io/O=company/C=US' -addext "subjectAltName = DNS:vanadium.io" > ${cafile}
}

newHost() {
    algo=$1
    host=$2
    shift;shift
    args=$*
    openssl ${algo} -out ${host}.key $args
    openssl req -new -key ${host}.key -out ${host}.csr \
        -subj "/CN=${host}/O=company/C=US" -addext "subjectAltName = DNS:${host}"

    openssl x509 -req \
        -CA ${cafile} -CAkey ${keyfile} -CAcreateserial  -copy_extensions copy \
        -in ${host}.csr \
        -out ${host}.crt -days 1825 -sha256 

    openssl verify -CAfile ${cafile}  ${host}.crt
}

genca
newHost genrsa rsa-2048.vanadium.io 2048
newHost genrsa rsa-4096.vanadium.io 4096
newHost ecparam ecdsa-256.vanadium.io -name prime256v1 -genkey
newHost ecparam ecdsa-384.vanadium.io -name prime256v1 -genkey
newHost ecparam ecdsa-521.vanadium.io -name prime256v1 -genkey
newHost genpkey ed25519.vanadium.io -algorithm ed25519

encryptFile() {
	algo=$1
	host=$2
	openssl $algo -aes256 -in ${host}.key -out encrypted.${host}.key -passout 'pass:password'
}

encryptFile rsa rsa-2048.vanadium.io
encryptFile rsa rsa-4096.vanadium.io
encryptFile ec ecdsa-256.vanadium.io
encryptFile ec ecdsa-384.vanadium.io
encryptFile ec ecdsa-521.vanadium.io
encryptFile pkey ed25519.vanadium.io
