
SHELL := /bin/bash
CURRENT_PATH = $(shell pwd)
APP_NAME = pier

# build with verison infos
VERSION_DIR = github.com/meshplus/${APP_NAME}
BUILD_DATE = $(shell date +%FT%T)
GIT_COMMIT = $(shell git log --pretty=format:'%h' -n 1)
GIT_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
ifeq (${GIT_BRANCH},HEAD)
  APP_VERSION = $(shell git describe --tags HEAD)
else
  APP_VERSION = dev
endif

GOLDFLAGS += -X "${VERSION_DIR}.BuildDate=${BUILD_DATE}"
GOLDFLAGS += -X "${VERSION_DIR}.CurrentCommit=${GIT_COMMIT}"
GOLDFLAGS += -X "${VERSION_DIR}.CurrentBranch=${GIT_BRANCH}"
GOLDFLAGS += -X "${VERSION_DIR}.CurrentVersion=${APP_VERSION}"

STATIC_LDFLAGS += ${GOLDFLAGS}
STATIC_LDFLAGS += -linkmode external -extldflags -static

GO = GO111MODULE=on go
TEST_PKGS := $(shell $(GO) list ./... | grep -v 'cmd' | grep -v 'mock_*' | grep -v 'proto' | grep -v 'imports' | grep -v 'internal/app' | grep -v 'api')
MODS = $(shell cat goent.diff | grep '^[^replace]' | tr '\n' '@')
GOFLAG = -mod=mod

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

packr2:
	cd internal/repo && packr2

prepare:
	cd scripts && bash prepare.sh

## make install: Go install the project (hpc)
install: packr2
	rm -f imports/imports.go
	$(GO) install $(GOFLAG) -ldflags '${GOLDFLAGS}' ./cmd/${APP_NAME}
	@printf "${GREEN}Install pier successfully${NC}\n"

build: packr2
	@mkdir -p bin
	rm -f imports/imports.go
	$(GO) build $(GOFLAG) -ldflags '${GOLDFLAGS}' ./cmd/${APP_NAME}
	@mv ./pier bin
	@printf "${GREEN}Build Pier successfully!${NC}\n"

installent: packr2
	cp imports/imports.go.template imports/imports.go
	@sed "s?)?$(MODS))?" go.mod  | tr '@' '\n' > goent.mod
	@cat goent.diff | grep '^replace' >> goent.mod
	$(GO) install $(GOFLAG) -tags ent -ldflags '${GOLDFLAGS}' -modfile goent.mod ./cmd/${APP_NAME}
	@printf "${GREEN}Install pier ent successfully${NC}\n"

buildent: packr2
	@mkdir -p bin
	cp imports/imports.go.template imports/imports.go
	@sed "s?)?$(MODS)@)?" go.mod  | tr '@' '\n' > goent.mod
	$(GO) build $(GOFLAG) -tags ent -ldflags '${GOLDFLAGS}' -modfile goent.mod ./cmd/${APP_NAME}
	@mv ./pier bin
	@printf "${GREEN}Build pier ent successfully!${NC}\n"

mod:
	sed "s?)?$(MODS)\n)?" go.mod

docker-build: packr2
	$(GO) install $(GOFLAG) -ldflags '${STATIC_LDFLAGS}' ./cmd/${APP_NAME}
	@echo "Build pier successfully"

## make build-linux: Go build linux executable file
build-linux:
	cd scripts && bash cross_compile.sh linux-amd64 ${CURRENT_PATH}

## make release: Build release before push
release-binary:
	@cd scripts && bash release_binary.sh

## make linter: Run golanci-lint
linter:
	golangci-lint run
