#!/bin/bash

# generate letsencrypt certs using DNS challenges via google cloud dns.

# google clound dns credentials file. The credentials file, which must
# be in json format is obtained by creating a new key for the service
# account via console.cloud.google.com.
service_account=$1

# update root and intermediate certs
for cert in letsencrypt-stg-root-x1.pem letsencrypt-stg-root-x2.pem; do
    curl -o ${cert} https://letsencrypt.org/certs/staging/${cert}
done

#for cert in e1.pem e2.pem r3.pem r4.peml; do
#    curl -o letsencrypt-stg-int-${cert} https://letsencrypt.org/certs/staging/letsencrypt-stg-int-${cert}
#done

run_certbot() {
type="$1"
to="$2"
shift; shift
domains=""
for d in $*; do
    domains="-d ${d} ${domains}"
done

workdir=$(pwd)/certbot-workdir
mkdir -p ${workdir}

chmod 0400 ${service_account}
cp ${service_account} ${workdir}/creds.json
docker run \
    --rm \
    --name certbot \
    -v "${workdir}:/etc/letsencrypt" \
    -v "${workdir}:/var/log/letsencrypt" \
    -v "${workdir}:/creds" \
  certbot/dns-google \
  certonly --test-cert \
    --dns-google \
    --dns-google-credentials /creds/creds.json \
    --key-type=${type} \
    --preferred-challenges dns \
    --email admin@cloudeng.io \
    --agree-tos --non-interactive \
    --cert-name=$to \
    ${domains}

  cp "${workdir}/live/${to}/fullchain.pem" ${to}.letsencrypt
  cp "${workdir}/live/${to}/privkey.pem" ${to}.letsencrypt.key
}

docker pull certbot/dns-google

run_certbot ecdsa www.labdrive.io 'www.labdrive.io'
run_certbot ecdsa abc.labdrive.io 'a.labdrive.io' 'b.labdrive.io' 'c.labdrive.io'
run_certbot rsa star.labdrive.io '*.labdrive.io'
run_certbot rsa ab-star.labdrive.io '*.labdrive.io' '*.labdr.io'
