#!/usr/bin/env bash

set -e
source x.sh

CURRENT_PATH=$(pwd)
PROJECT_PATH=$(dirname "${CURRENT_PATH}")
RELEASE_PATH=${PROJECT_PATH}/bin
APP_VERSION=$(if [ `git rev-parse --abbrev-ref HEAD` == 'HEAD' ];then git describe --tags HEAD ; else echo "dev" ; fi)

print_blue "===> 1. Install packr"
if ! type packr >/dev/null 2>&1; then
  go get -u github.com/gobuffalo/packr/packr
fi

print_blue "===> 2. build pier"
cd "${PROJECT_PATH}" && make build

print_blue "===> 3. pack binarys"
cd "${RELEASE_PATH}"
if [ "$(uname)" == "Darwin" ]; then
  cp ../build/wasm/lib/darwin-amd64/libwasmer.dylib .
  tar zcvf pier_darwin_x86_64_"${APP_VERSION}".tar.gz ./pier ./libwasmer.dylib
else
  cp ../build/wasm/lib/linux-amd64/libwasmer.so .
  tar zcvf pier_linux-amd64_"${APP_VERSION}".tar.gz ./pier ./libwasmer.so
fi
