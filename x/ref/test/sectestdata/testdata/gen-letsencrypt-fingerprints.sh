#!/bin/bash

for file in *.letsencrypt letsencrypt-stg-*.pem; do
    rm -f ${file}.fingeprint
done

for file in *.letsencrypt letsencrypt-stg-*.pem; do
    openssl x509 -in $file --pubkey --noout |
    openssl ec --pubin --inform PEM --outform DER |openssl md5 -c |
    sed 's/MD5(stdin)= //' > ${file}.fingerprint
done
