#!/usr/bin/env bash

set -e
source x.sh

CURRENT_PATH=$(pwd)
PROJECT_PATH=$(dirname "${CURRENT_PATH}")
RELEASE_PATH=${PROJECT_PATH}/bin
APP_VERSION=$(if [ `git rev-parse --abbrev-ref HEAD` == 'HEAD' ];then git describe --tags HEAD ; else echo "dev" ; fi)

function go_install() {
  version=$(go env GOVERSION)
  if [[ ! "$version" < "go1.16" ]];then
      go install "$@"
  else
      go get "$@"
  fi
}
print_blue "===> 1. Install packr2"
if ! type packr2 >/dev/null 2>&1; then
  go_install github.com/gobuffalo/packr/v2/packr2@v2.8.3
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
