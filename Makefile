# Copyright 2014-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You
# may not use this file except in compliance with the License. A copy of
# the License is located at
#
#       http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is
# distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
# ANY KIND, either express or implied. See the License for the specific
# language governing permissions and limitations under the License.
#

GOVERSION=1.9.3
UNIQUE:=$(shell date +%s)
MAKEDIR:=$(strip $(shell dirname "$(realpath $(lastword $(MAKEFILE_LIST)))"))


# build binary
static:
	go build -o .build/aws-k8s-agent cmd/ipamd/main.go
	go build -o .build/aws-cni cmd/cni/cni.go
	# go build verify-aws.go
	# go build verify-network.go

# need to bundle certificates
certs: misc/certs/ca-certificates.crt
misc/certs/ca-certificates.crt:
	docker build -t "amazon/amazon-k8s-cert-source:make" misc/certs/
	docker run "amazon/amazon-k8s-cert-source:make" cat /etc/ssl/certs/ca-certificates.crt > misc/certs/ca-certificates.crt


# build docker image
docker-release: certs
	@docker build -f scripts/dockerfiles/Dockerfile.release -t "amazon/amazon-k8s-cni:latest" .
	@echo "Built Docker image \"amazon/amazon-k8s-cni:latest\""

# unit-test
unit-test:
	# go test -v -cover -race -timeout 60s ./pkg/awsutils/...
	# go test -v -cover -race -timeout 10s ./cmd/cni/...
	# go test -v -cover -race -timeout 10s ./cmd/cni/driver
	# go test -v -cover -race -timeout 10s ./pkg/k8sapi/...
	# go test -v -cover -race -timeout 10s ./pkg/networkutils/...
	go test -v -cover -race -timeout 60s ./pkg/...
	go test -v -cover -race -timeout 10s ./cmd/...
	go test -v -cover -race -timeout 10s ./ipamd/...

#golint
lint:
	golint pkg/awsutils/*.go
	golint cmd/cni/*.go
	golint cmd/cni/driver/*.go
	golint pkg/k8sapi/*.go
	golint pkg/networkutils/*.go
	golint ipamd/*.go
	golint ipamd/*/*.go

#go tool vet
vet:
	go tool vet ./pkg/awsutils
	go tool vet ./cmd/cni
	go tool vet ./pkg/k8sapi
	go tool vet ./pkg/networkutils

.PHONY: docker
docker:
	docker pull golang:${GOVERSION} # Keep golang image up to date
	docker run --name=amazon-vpc-cni-k8s-${UNIQUE} -e STATIC_BUILD=yes -e VERSION=${VERSION} -v ${MAKEDIR}:/go/src/github.com/aws/amazon-vpc-cni-k8s golang:${GOVERSION} make -C /go/src/github.com/aws/amazon-vpc-cni-k8s/ static
	docker cp amazon-vpc-cni-k8s-${UNIQUE}:/go/src/github.com/aws/amazon-vpc-cni-k8s/.build .

.PHONY: docker
docker-unit-test:
	docker pull golang:${GOVERSION} # Keep golang image up to date
	docker run --name=amazon-vpc-cni-k8s-${UNIQUE} -e STATIC_BUILD=yes -e VERSION=${VERSION} -v ${MAKEDIR}:/go/src/github.com/aws/amazon-vpc-cni-k8s golang:${GOVERSION} make -C /go/src/github.com/aws/amazon-vpc-cni-k8s/ unit-test
