#!/bin/bash

# generate letsencrypt certs using DNS challenges via google cloud dns.

# google clound dns credentials file. The credentials file, which must
# be in json format is obtained by creating a new key for the service
# account via console.cloud.google.com.
service_account=$1

# update root certs
for cert in letsencrypt-stg-int-r3.pem letsencrypt-stg-int-e1.pem; do
    curl -o ${cert} https://letsencrypt.org/certs/staging/${cert}
done

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

run_certbot ecdsa www.labdrive.io 'www.labdrive.io'
run_certbot ecdsa abc.labdrive.io 'a.labdrive.io' 'b.labdrive.io' 'c.labdrive.io'
run_certbot rsa star.labdrive.io '*.labdrive.io'
run_certbot rsa ab-star.labdrive.io '*.labdrive.io' '*.labdr.io'
