
openssl genrsa 2048 > root-key.pem
openssl req -x509 -new -nodes -key root-key.pem -sha256 -days 1825 -subj '/CN=vanadium.io/O=company/C=US' > root-ca.pem

newHost() {
    algo=$1
    host=$2
    shift;shift
    args=$*
    openssl ${algo} -out ${host}.key $args
    openssl req -new -key ${host}.key -out ${host}.csr \
        -subj "/CN=${host}/O=company/C=US"

    openssl x509 -req \
        -CA root-ca.pem -CAkey root-key.pem -CAcreateserial \
        -in ${host}.csr \
        -out ${host}.crt -days 1825 -sha256 

    openssl verify -CAfile root-ca.pem  ${host}.crt
}

newHost genrsa rsa2048.vanadium.io 2048
newHost genrsa rsa4096.vanadium.io 4096
newHost ecparam ec256.vanadium.io -name prime256v1 -genkey
newHost genpkey ed25519.vanadium.io -algorithm ed25519
