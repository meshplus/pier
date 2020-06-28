#!/usr/bin/env bash

set -e

source x.sh

# $1 is arch, $2 is source code path
case $1 in
linux-amd64)
  print_blue "Compile for linux/amd64"
  docker run -t \
    -v $2:/code/pier \
    -v $2/../pier-client-fabric:/code/pier-client-fabric \
    -v $2/../pier-client-ethereum:/code/pier-client-ethereum \
    -v ~/.ssh:/root/.ssh \
    -v ~/.gitconfig:/root/.gitconfig \
    -v $GOPATH/pkg/mod:$GOPATH/pkg/mod \
    pier-ubuntu/compile \
    /bin/bash -c "go env -w GO111MODULE=on &&
      go env -w GOPROXY=https://goproxy.cn,direct &&
      go get -u github.com/gobuffalo/packr/packr &&
      cd /code/pier-client-fabric && make fabric1.4 &&
      cd /code/pier-client-ethereum && make eth &&
      cd /code/pier && make install &&
      mkdir -p /code/pier/bin &&
      cp /go/bin/pier /code/pier/bin/pier_linux-amd64 &&
      cp /code/pier-client-fabric/build/fabric-client-1.4.so /code/pier/bin/pier-fabric-linux.so &&
      cp /code/pier-client-ethereum/build/eth-client.so /code/pier/bin/pier-eth-linux.so"
  ;;
*)
  print_red "Other architectures are not supported yet"
  ;;
esac
