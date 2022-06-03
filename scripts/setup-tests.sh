#!/usr/bin/env bash

cat <<EOF > _out/openssl.conf
[ req ]
distinguished_name = subject
x509_extensions    = cert_ext

[ subject ]
commonName      = Common Name (e.g. server FQDN or YOUR name)
commonName_max  = 64

[ cert_ext ]
subjectAltName = @alternate_names

[ alternate_names ]
DNS.1 = localhost
IP.1  = 127.0.0.1
IP.2  = ::1
EOF

openssl req -x509 -config _out/openssl.conf -newkey rsa:4096 -keyout _out/key.pem -out _out/cert.pem -sha256 -days 30 -nodes -subj '/CN=localhost'

mkdir -p _out/testdata/tls
cp testdata/pdns/test/tls/apikey.yml _out/testdata/tls/apikey.yml
sed "s#__CERT__#$(base64 -w0 _out/cert.pem)#g" testdata/pdns/test/tls/config.json > _out/testdata/tls/config.json

# No TLS
mkdir -p _out/testdata/no-tls
cp testdata/pdns/test/no-tls/{config.json,apikey.yml} _out/testdata/no-tls
