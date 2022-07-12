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
DNS.2 = web
IP.1  = 127.0.0.1
IP.2  = ::1
EOF

openssl req -x509 -config _out/openssl.conf -newkey rsa:4096 -keyout _out/key.pem -out _out/cert.pem -sha256 -days 30 -nodes -subj '/CN=localhost'

for suite in tls tls-with-proxy; do
  mkdir -p _out/testdata/${suite}
  cp testdata/pdns/test/${suite}/apikey.yml _out/testdata/${suite}/apikey.yml
  sed "s#__CERT__#$(base64 -w0 _out/cert.pem)#g" testdata/pdns/test/${suite}/config.json > _out/testdata/${suite}/config.json
done

# No TLS
for suite in no-tls no-tls-with-proxy; do
  mkdir -p _out/testdata/${suite}
  cp testdata/pdns/test/${suite}/{config.json,apikey.yml} _out/testdata/${suite}
done
