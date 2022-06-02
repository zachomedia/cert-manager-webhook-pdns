IMAGE_NAME := "zachomedia/cert-manager-webhook-pdns"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out

$(shell mkdir -p "$(OUT)")

setup:
	./scripts/fetch-test-binaries.sh

verify:
	TEST_ASSET_ETCD=_out/kubebuilder/bin/etcd TEST_ASSET_KUBE_APISERVER=_out/kubebuilder/bin/kube-apiserver TEST_ASSET_KUBECTL=_out/kubebuilder/bin/kubectl TEST_DNS_SERVER="127.0.0.1:53" TEST_ZONE_NAME=example.ca. go test .

test: verify

build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: rendered-manifest.yaml build verify test setup
rendered-manifest.yaml:
	helm template \
        --set image.repository=$(IMAGE_NAME) \
        --set image.tag=$(IMAGE_TAG) \
	      cert-manager-webhook-pdns \
        deploy/cert-manager-webhook-pdns > "$(OUT)/rendered-manifest.yaml"
