#!/usr/bin/env bash

set -e
source x.sh

CURRENT_PATH=$(pwd)
PROJECT_PATH=$(dirname "${CURRENT_PATH}")
RELEASE_PATH=${PROJECT_PATH}/bin
APP_VERSION=${1:-'1.6.0'}

function checkout_tag() {
  if [ `git tag -l |grep v${APP_VERSION}` ]; then
    git checkout v"${APP_VERSION}"
  else
    print_blue "plugins tag does not exist"
    exit 1
  fi
}

print_blue "===> 1. Install packr"
if ! type packr >/dev/null 2>&1; then
  go get -u github.com/gobuffalo/packr/packr
fi

print_blue "===> 2. build pier"
cd "${PROJECT_PATH}" && make build

print_blue "===> 2. build pier-client-fabric and pier-client-ethereum"
cd "${RELEASE_PATH}" && git clone https://github.com/meshplus/pier-client-fabric.git
cd pier-client-fabric && checkout_tag && make fabric1.4

cd "${RELEASE_PATH}" && git clone https://github.com/meshplus/pier-client-ethereum.git
cd pier-client-ethereum && checkout_tag && make eth

print_blue "===> 3. pack binarys"
cd "${RELEASE_PATH}"
cp ./pier-client-fabric/build/fabric-client* .
cp ./pier-client-ethereum/build/eth-client* .
cp ../build/libwasmer.so .
if [ "$(uname)" == "Darwin" ]; then
    tar zcvf pier_v"${APP_VERSION}"_Darwin_x86_64.tar.gz ./pier ./*.so ./fabric-client* ./eth-client*
else
    tar zcvf pier_v"${APP_VERSION}"_Linux_x86_64.tar.gz ./pier ./*.so ./fabric-client* ./eth-client*
fi

#print_blue "===> 4. clear useless files"
#cd "${RELEASE_PATH}"
#ls |grep -v *.tar.gz |xargs rm -rf