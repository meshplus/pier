#!/usr/bin/env bash

set -e
source x.sh

CURRENT_PATH=$(pwd)
PROJECT_PATH=$(dirname "${CURRENT_PATH}")
RELEASE_PATH=${PROJECT_PATH}/bin
APP_VERSION=${1:-'1.6.0'}

print_blue "===> 1. Install packr"
if ! type packr >/dev/null 2>&1; then
  go get -u github.com/gobuffalo/packr/packr
fi

print_blue "===> 2. build pier"
cd "${PROJECT_PATH}" && make build

print_blue "===> 3. pack binarys"
cd "${RELEASE_PATH}"
if [ "$(uname)" == "Darwin" ]; then
  cp ../build/libwasmer.dylib .
  tar zcvf pier_v"${APP_VERSION}"_Darwin_x86_64.tar.gz ./pier ./libwasmer.dylib
else
  cp ../build/libwasmer.so .
  tar zcvf pier_v"${APP_VERSION}"_Linux_x86_64.tar.gz ./pier ./libwasmer.so
fi
