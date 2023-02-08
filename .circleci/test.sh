#!/bin/bash

set -ex

# Install Go
curl -LO https://go.dev/dl/go1.19.5.linux-amd64.tar.gz

tar -C /usr/local -xzf go1.19.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
go version

# Install Kustomize
curl -LO https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv3.10.0/kustomize_v3.10.0_linux_amd64.tar.gz
tar -zxvf kustomize_v3.10.0_linux_amd64.tar.gz
chmod +x kustomize
mv kustomize /usr/local/bin/kustomize

# Run tests
cd /saffire/
make all install
USE_EXISTING_CLUSTER=true make test
