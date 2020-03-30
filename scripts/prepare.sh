#!/usr/bin/env bash

printf "1. Set go proxy\n"
export GO111MODULE=on
go env -w GOPROXY=https://goproxy.cn,direct

printf "2. Install packr\n"
go get -u github.com/gobuffalo/packr/packr

printf "3. Install golangci-lint\n"
go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.18.0
