#!/usr/bin/make -f
DOCKER := $(shell which docker)

###############################################################################
###                                Building                                 ###
###############################################################################

build: clean
	go build -o build/ ./

build-linux:
	GOOS=linux GOARCH=$(if $(findstring aarch64,$(shell uname -m)) || $(findstring arm64,$(shell uname -m)),arm64,amd64) $(MAKE) build

.PHONY: build build-linux

###############################################################################
###                                Linting                                  ###
###############################################################################

golangci_lint_cmd=$$(go env GOPATH)/bin/golangci-lint
golangci_version=v1.49.0

# look into .golangci.yml for enabling / disabling linters
lint:
	@echo "--> Running linter"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run --timeout=10m

lint-fix:
	@echo "--> Running linter"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run --fix --out-format=tab --issues-exit-code=0

.PHONY: lint lint-fix

# Clean up the build directory
clean:
	rm -rf build/

.PHONY: clean



###############################################################################
###                                Protobuf                                 ###
###############################################################################


protoVer=0.14.0
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace $(protoImageName)

# ------
# NOTE: If you are experiencing problems running these commands, try deleting
#       the docker images and execute the desired command again.
#
proto-all: proto-format proto-lint proto-gen

proto-gen:
	@echo "Generating Protobuf files"
	$(protoImage) sh ./scripts/protocgen.sh
proto-lint:
	@echo "Linting Protobuf files"
	@$(protoImage) buf lint --error-format=json

proto-check-breaking:
	@echo "Checking Protobuf files for breaking changes"
	@$(protoImage) buf breaking --against $(HTTPS_GIT)#branch=main

proto-format:
	@echo "Formatting Protobuf files"
	@$(protoImage) find ./ -name "*.proto" -exec clang-format -i {} \;

.PHONY: proto-all proto-gen proto-format proto-lint proto-check-breaking

