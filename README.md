# PowerDNS cert-manager ACME webhook

## Installing

To install with helm, run:

```bash
$ helm repo add cert-manager-webhook-pdns https://zachomedia.github.io/cert-manager-webhook-pdns
$ helm install cert-manager-webhook-pdns cert-manager-webhook-pdns/cert-manager-webhook-pdns
```

Without helm, run:

```bash
$ make rendered-manifest.yaml
$ kubectl apply -f _out/rendered-manifest.yaml
```

### Issuer/ClusterIssuer

An example issuer:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pdns-api-key
type: Opaque
data:
  key: APIKEY_BASE64
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    email: certificates@example.ca
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-staging-account-key
    solvers:
      - dns01:
          webhook:
            groupName: acme.zacharyseguin.ca
            solverName: pdns
            config:
              # Base URL of the PowerDNS server.
              host: https://ns1.example.ca

              # Reference to the Kubernetes secret containing the API key.
              apiKeySecretRef:
                name: pdns-api-key
                key: key

              ###
              ### OPTIONAL
              ###

              # Server ID for the PowerDNS API.
              # When unset, defaults to "localhost".
              serverID: localhost

              # Request headers when connecting to the PowerDNS API.
              # The following headers are set by default, but can be overriden:
              #   X-API-Key
              #   Content-Type
              headers:
                key: value

              # CA bundle for TLS connections
              # When unset, the default system certificate store is used.
              caBundle: BASE64_ENCODED_CA_BUNDLE

              # TTL for DNS records
              # (in seconds)
              ttl: 120

              # Timeout for requests to the PDNS api server
              # (in seconds)
              timeout: 30
```

And then you can issue a cert:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: test-example-ca
  namespace: default
spec:
  secretName: example-com-tls
  dnsNames:
  - example.ca
  - www.example.ca
  issuerRef:
    name: letsencrypt-staging
    kind: Issuer
    group: cert-manager.io
```

## Development

### Running the test suite

You can run the test suite with:

1. `make setup`
2. `make test`

This requires `openssl`, `docker` and `docker-compose` to be installed.
