#!/bin/bash
version="v1.3.1"

flags="-X 'main.version=$version' -X 'main.goVersion=$(go version)' -X 'main.buildTime=$(date +"%Y-%m-%d_%H:%m:%S")' -X main.GitCommit=$(git rev-parse HEAD) "
go mod tidy
go build -ldflags "$flags" -o queryCollector main.go
test $? -eq 0 &&echo "build Success. Please use ./queryCollector -version for test."
