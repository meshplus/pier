#!/usr/bin/env bash

BLUE='\033[0;34m'
NC='\033[0m'

function print_blue() {
  printf "${BLUE}%s${NC}\n" "$1"
}

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

print_blue "===> 2. Install golangci-lint"
if ! type golanci-lint >/dev/null 2>&1; then
  go_install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.23.0
fi

print_blue "===> 3. Install go mock tool"
if ! type gomock >/dev/null 2>&1; then
  go_install github.com/golang/mock/gomock
fi
if ! type mockgen >/dev/null 2>&1; then
  go_install github.com/golang/mock/mockgen
fi