BINARY     := typesense-snapshot-controller
IMAGE_REPO ?= ghcr.io/opentelekomcloud/typesense-snapshot-controller
IMAGE_TAG  ?= latest

.PHONY: build test lint docker-build helm-lint

build:
	CGO_ENABLED=0 go build -o bin/$(BINARY) ./cmd/...

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

docker-build:
	docker build -t $(IMAGE_REPO):$(IMAGE_TAG) .

helm-lint:
	helm lint deploy/helm/typesense-snapshot-controller
