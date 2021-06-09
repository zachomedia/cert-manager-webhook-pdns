# PowerDNS cert-manager ACME webhook

## Installing

To install with helm, run:

```bash
$ git clone https://github.com/zachomedia/cert-manager-webhook-pdns.git
$ cd cert-manager-webhook-pdns/deploy/cert-manager-webhook-pdns
$ helm install --name cert-manager-webhook-pdns .
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
apiVersion: cert-manager.io/v1alpha3
kind: Issuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    email: certmaster@example.com
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-staging-account-key
    solvers:
    - dns01:
        webhook:
          groupName: acme.zacharyseguin.ca
          solverName: pdns
          config:
            host: https://ns1.example.com
            apiKeySecretRef:
              name: pdns-api-key
              key: key

            # Optional config, shown with default values
            #   all times in seconds
            ttl: 120
            timeout: 30
            propagationTimeout: 120
            pollingInterval: 2
```

And then you can issue a cert:

```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  name: test-zacharyseguin-ca
  namespace: default
spec:
  secretName: example-com-tls
  commonName: example.com
  dnsNames:
  - example.com
  - www.example.com
  issuerRef:
    name: letsencrypt-staging
    kind: Issuer
  acme:
    config:
      - dns01:
          provider: dns
        domains:
          - example.com
          - www.example.com

```

## Development

### Running the test suite

You can run the test suite with:

1. Copy `testdata/pdns/apikey.yml.sample` and `testdata/pdns/config.json.sample` and fill in the appropriate values

```bash
$ ./scripts/fetch-test-binaries.sh
$ TEST_ZONE_NAME=example.com. go test .
```
