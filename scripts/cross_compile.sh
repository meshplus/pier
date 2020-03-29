#!/usr/bin/env bash

set -e

source x.sh

# $1 is arch, $2 is source code path
case $1 in
linux-amd64)
  print_blue "Compile for linux/amd64"
  # docker pull golang:1.13
  docker run -t \
    -v $2:/code/pier \
    -v ~/.ssh:/root/.ssh \
    -v ~/.gitconfig:/root/.gitconfig \
    -v $GOPATH/pkg/mod:$GOPATH/pkg/mod \
    pier-ubuntu/compile \
    /bin/bash -c "go env -w GO111MODULE=on &&
      go env -w GOPROXY=https://goproxy.cn,direct &&
      go get -u github.com/gobuffalo/packr/packr &&
      cd /code/pier &&
      make install &&
      mkdir -p /code/pier/bin &&
      cp /go/bin/pier /code/pier/bin/pier_linux-amd64"
  ;;
*)
  print_red "Other architectures are not supported yet"
  ;;
esac
