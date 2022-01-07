
set -e

keyfile=vanadium.io.key.pem
cafile=vanadium.io.ca.pem
openssl genrsa 2048 > ${keyfile}
openssl req -x509 -new -nodes -key ${keyfile} -sha256 -days 1825 -subj '/CN=vanadium.io/O=company/C=US' > ${cafile}

newHost() {
    algo=$1
    host=$2
    shift;shift
    args=$*
    openssl ${algo} -out ${host}.key $args
    openssl req -new -key ${host}.key -out ${host}.csr \
        -subj "/CN=${host}/O=company/C=US"

    openssl x509 -req \
        -CA ${cafile} -CAkey ${keyfile} -CAcreateserial \
        -in ${host}.csr \
        -out ${host}.crt -days 1825 -sha256 

    openssl verify -CAfile ${cafile}  ${host}.crt
}

newHost genrsa rsa2048.vanadium.io 2048
newHost genrsa rsa4096.vanadium.io 4096
newHost ecparam ec256.vanadium.io -name prime256v1 -genkey
newHost genpkey ed25519.vanadium.io -algorithm ed25519
