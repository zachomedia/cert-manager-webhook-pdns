IMAGE_NAME := "zachomedia/cert-manager-webhook-pdns"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/_out

$(shell mkdir -p "$(OUT)")

verify:
	go test -v .

build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
        --set image.repository=$(IMAGE_NAME) \
        --set image.tag=$(IMAGE_TAG) \
	      cert-manager-webhook-pdns \
        deploy/cert-manager-webhook-pdns > "$(OUT)/rendered-manifest.yaml"
