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
              host: https://ns1.example.ca
              apiKeySecretRef:
                name: pdns-api-key
                key: key

              ###
              ### OPTIONAL
              ###

              # CA bundle for TLS connections
              # When unset,
              caBundle: BASE64_ENCODE_CA_BUNDLE

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

1. Copy `testdata/pdns/apikey.yml.sample` and `testdata/pdns/config.json.sample` and fill in the appropriate values

2. Run tests
```bash
$ ./scripts/fetch-test-binaries.sh
$ TEST_ASSET_ETCD=_out/kubebuilder/bin/etcd TEST_ASSET_KUBE_APISERVER=_out/kubebuilder/bin/kube-apiserver TEST_ASSET_KUBECTL=_out/kubebuilder/bin/kubectl TEST_ZONE_NAME=example.com. go test .
```

It is possible to use an alternative DNS-Server to check for propagation - just set the ENV variable TEST_DNS_SERVER accordingly

```bash
$ TEST_ASSET_ETCD=_out/kubebuilder/bin/etcd TEST_ASSET_KUBE_APISERVER=_out/kubebuilder/bin/kube-apiserver TEST_ASSET_KUBECTL=_out/kubebuilder/bin/kubectl TEST_DNS_SERVER="192.168.1.1:53" TEST_ZONE_NAME=example.com. go test .
```
