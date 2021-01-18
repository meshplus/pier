
SHELL := /bin/bash
CURRENT_PATH = $(shell pwd)
APP_NAME = pier
APP_VERSION = 1.0.0

# build with verison infos
VERSION_DIR = github.com/meshplus/${APP_NAME}
BUILD_DATE = $(shell date +%FT%T)
GIT_COMMIT = $(shell git log --pretty=format:'%h' -n 1)
GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)

GOLDFLAGS += -X "${VERSION_DIR}.BuildDate=${BUILD_DATE}"
GOLDFLAGS += -X "${VERSION_DIR}.CurrentCommit=${GIT_COMMIT}"
GOLDFLAGS += -X "${VERSION_DIR}.CurrentBranch=${GIT_BRANCH}"
GOLDFLAGS += -X "${VERSION_DIR}.CurrentVersion=${APP_VERSION}"

STATIC_LDFLAGS += ${GOLDFLAGS}
STATIC_LDFLAGS += -linkmode external -extldflags -static

GO = GO111MODULE=on go
TEST_PKGS := $(shell $(GO) list ./... | grep -v 'cmd' | grep -v 'mock_*')

RED=\033[0;31m
GREEN=\033[0;32m
BLUE=\033[0;34m
NC=\033[0m

.PHONY: test

help: Makefile
	@echo "Choose a command run:"
	@sed -n 's/^##//p' $< | column -t -s ':' | sed -e 's/^/ /'

## make test: Run go unittest
test:
	go generate ./...
	@$(GO) test ${TEST_PKGS} -count=1

## make test-coverage: Test project with cover
test-coverage:
	go generate ./...
	@go test -short -coverprofile cover.out -covermode=atomic ${TEST_PKGS}
	@cat cover.out >> coverage.txt

packr:
	cd internal/repo && packr

prepare:
	cd scripts && bash prepare.sh

## make install: Go install the project (hpc)
install: packr
	rm -f imports/imports.go
	$(GO) install -ldflags '${GOLDFLAGS}' ./cmd/${APP_NAME}
	@printf "${GREEN}Build pier successfully${NC}\n"

build: packr
	@mkdir -p bin
	rm -f imports/imports.go
	$(GO) build -ldflags '${GOLDFLAGS}' ./cmd/${APP_NAME}
	@mv ./pier bin
	@printf "${GREEN}Build Pier successfully!${NC}\n"

installent: packr
	cp imports/imports.go.template imports/imports.go
	@sed "s?)?$(MODS)@)?" go.mod  | tr '@' '\n' > goent.mod
	$(GO) install -tags ent -ldflags '${GOLDFLAGS}' -modfile goent.mod ./cmd/${APP_NAME}

buildent: packr
	@mkdir -p bin
	cp imports/imports.go.template imports/imports.go
	@sed "s?)?$(MODS)@)?" go.mod  | tr '@' '\n' > goent.mod
	$(GO) build -tags ent -ldflags '${GOLDFLAGS}' -modfile goent.mod ./cmd/${APP_NAME}
	@mv ./pier bin
	@printf "${GREEN}Build pier ent successfully!${NC}\n"

mod:
	sed "s?)?$(MODS)\n)?" go.mod

docker-build: packr
	$(GO) install -ldflags '${STATIC_LDFLAGS}' ./cmd/${APP_NAME}
	@echo "Build pier successfully"

## make build-linux: Go build linux executable file
build-linux:
	cd scripts && bash cross_compile.sh linux-amd64 ${CURRENT_PATH}

## make linter: Run golanci-lint
linter:
	golangci-lint run
