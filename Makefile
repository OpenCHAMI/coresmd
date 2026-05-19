# SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

# Set path to commands
GO            ?= $(shell command -v go 2>/dev/null)
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null)
GORELEASER    ?= $(shell command -v goreleaser 2>/dev/null)
GIT           ?= $(shell command -v git 2>/dev/null)
AWK           ?= $(shell command -v awk 2>/dev/null)
REUSE         ?= $(shell command -v reuse 2>/dev/null)
COREDHCP_GEN  ?= $(shell command -v coredhcp-generator)
# Use HOSTCMD to not conflict with Make's $(HOSTNAME)
HOSTCMD        ?= $(shell command -v hostname 2>/dev/null)
INSTALL        ?= $(shell command -v install 2>/dev/null)
SCDOC          ?= $(shell command -v scdoc 2>/dev/null)
CONTAINER_PROG ?= $(shell command -v docker 2>/dev/null)
SHELL          ?= /bin/sh

INSTALL_PROGRAM ?= $(INSTALL) -Dm755
INSTALL_DATA    ?= $(INSTALL) -Dm644

# Check that commands are present
ifeq ($(GIT),)
$(error git command not found.)
endif
ifeq ($(HOSTCMD),)
$(error hostname command not found.)
endif
ifeq ($(SHELL),)
$(error '$(SHELL)' undefined.)
endif

# Recursive wildcard function, obtained from https://stackoverflow.com/a/18258352
#
# Arg 1: Space-separated list of directories to recurse into
# Arg 2: Space-separated list of patterns to match
rwildcard = $(foreach d,$(wildcard $(1:=/*)),$(call rwildcard,$d,$2) $(filter $(subst *,%,$2),$d))

NAME          ?= coresmd
IMPORT        := github.com/openchami/$(NAME)
CONTAINER_TAG ?= latest
FQCN          := ghcr.io/openchami/$(NAME):$(CONTAINER_TAG)
VERSION       ?= $(shell $(GIT) describe --tags --always --dirty --broken --abbrev=0)
TAG           ?= $(shell $(GIT) describe --tags --always --abbrev=0)
BRANCH        ?= $(shell $(GIT) branch --show-current)
BUILD         ?= $(shell $(GIT) rev-parse HEAD)
GOVER         := $(shell $(GO) env GOVERSION)
GITSTATE      := $(shell if output=$($(GIT) status --porcelain) && [ -n "$output" ]; then echo dirty; else echo clean; fi)
BUILDHOST     := $(shell $(HOSTCMD))
BUILDUSER     := $(shell whoami)
BUILD_IS_PR   ?= 0
LDFLAGS := -s \
	   -X '$(IMPORT)internal/version.Version=$(VERSION)' \
	   -X '$(IMPORT)internal/version.Tag=$(TAG)' \
	   -X '$(IMPORT)internal/version.Branch=$(BRANCH)' \
	   -X '$(IMPORT)internal/version.Commit=$(BUILD)' \
	   -X '$(IMPORT)internal/version.Date=$(shell date -Iseconds)' \
	   -X '$(IMPORT)internal/version.GoVersion=$(GOVER)' \
	   -X '$(IMPORT)internal/version.GitState=$(GITSTATE)' \
	   -X '$(IMPORT)internal/version.BuildHost=$(BUILDHOST)' \
	   -X '$(IMPORT)internal/version.BuildUser=$(BUILDUSER)'

CMD      := $(call rwildcard,cmd,*.go)
INTERNAL := $(call rwildcard,internal,*.go)
PKG      := $(call rwildcard,pkg,*.go)
MANSRC   := $(wildcard man/*.sc)
MANBIN   := $(subst .sc,,$(MANSRC))
MAN1BIN  := $(filter %.1,$(MANBIN))
MAN5BIN  := $(filter %.5,$(MANBIN))

HELPERS :=

prefix      ?= /usr/local
exec_prefix ?= $(prefix)
bindir      ?= $(exec_prefix)/bin
mandir      ?= $(exec_prefix)/man
libexecdir  ?= $(prefix)/usr/libexec/$(NAME)
sharedir    ?= $(prefix)/usr/share

.PHONY: all
all: binaries ## Build everything

.PHONY: binaries
binaries: coredhcp coredns ## Build binaries

.PHONY: container
container: coredhcp coredns ## Build container
ifeq ($(CONTAINER_PROG),)
	$(error specified container command ($(CONTAINER_PROG)) not found)
endif
	$(CONTAINER_PROG) build -t $(FQCN) .

.PHONY: container-multistage
container-multistage: ## Build container using multi-stage Dockerfile
ifeq ($(CONTAINER_PROG),)
	$(error specified container command ($(CONTAINER_PROG)) not found)
endif
	$(CONTAINER_PROG) build -t $(FQCN) -f Dockerfile.build .

.PHONY: help
help: ## Show this help
ifeq ($(AWK),)
        $(error awk command not found.)
endif
	@$(AWK) 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m[VAR=val]... <target>\033[0m\n\nTargets:\n"} \
	/^[a-zA-Z0-9_\/.-]+:.*##/ { \
	        printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 \
	}' $(MAKEFILE_LIST)

.PHONY: goreleaser-build
goreleaser-build: ## Run `goreleaser build` (accepts GORELEASER_OPTS)
ifeq ($(GO),)
	$(error go command not found.)
endif
ifeq ($(GORELEASER),)
	$(error goreleaser command not found.)
endif
	env \
		IS_PR_BUILD=$(BUILD_IS_PR) \
		GO_VERSION=$(GOVER) \
		BUILD_HOST=$(BUILDHOST) \
		BUILD_USER=$(BUILDUSER) \
		$(GORELEASER) build $(GORELEASER_OPTS)

.PHONY: goreleaser-release
goreleaser-release: ## Run `goreleaser release` (accepts GORELEASER_OPTS)
ifeq ($(GO),)
	$(error go command not found.)
endif
ifeq ($(GORELEASER),)
	$(error goreleaser command not found.)
endif
	env \
		IS_PR_BUILD=$(BUILD_IS_PR) \
		GO_VERSION=$(GOVER) \
		BUILD_HOST=$(BUILDHOST) \
		BUILD_USER=$(BUILDUSER) \
		$(GORELEASER) release $(GORELEASER_OPTS)

.PHONY: goreleaser-clean
goreleaser-clean: ## Clean Goreleaser files (remove dist/)
	$(RM) -rf dist/

.PHONY: check-reuse
check-reuse: ## Check REUSE compliance
ifeq ($(REUSE),)
	$(error reuse command not found)
endif
	reuse lint --lines

.PHONY: lint
lint:
ifeq ($(GOLANGCI_LINT),)
	$(error golangci-lint command not found)
endif
	$(GOLANGCI_LINT) run

.PHONY: test
test: unit-test ## Run all tests

.PHONY: unit-test
unit-test: ## Run unit tests only
ifeq ($(GO),)
	$(error go command not found.)
endif
	$(GO) test -cover -v ./...

.PHONY: clean
clean: clean-coredhcp clean-coredns ## Clean Go build artifacts

.PHONY: clean-coredhcp
clean-coredhcp: ## Clean coredhcp binary and generated Go files
	$(GO) clean -i -x ./build/coredhcp
	$(RM) ./build/coredhcp/coredhcp.go
	$(RM) coredhcp

.PHONY: clean-coredns
clean-coredns: ## Clean coredns binary
	$(GO) clean -i -x ./build/coredns
	$(RM) coredns

.PHONY: install
install: install ## Install everything

.PHONY: install-coredhcp
install-coredhcp: coredhcp ## Install CoreDHCP
ifeq ($(INSTALL),)
	$(error install command not found.)
endif
	$(INSTALL_PROGRAM) $< $(DESTDIR)$(bindir)/$<

.PHONY: install-coredns
install-coredns: coredns ## Install CoreDNS
ifeq ($(INSTALL),)
	$(error install command not found.)
endif
	$(INSTALL_PROGRAM) $< $(DESTDIR)$(bindir)/$<

.PHONY: uninstall
uninstall: uninstall-coredhcp uninstall-coredns ## Uninstall everything

.PHONY: uninstall-coredhcp
uninstall-coredhcp: ## Uninstall CoreDHCP
	rm -f $(DESTDIR)$(bindir)/coredhcp

.PHONY: uninstall-coredns
uninstall-coredns: ## Uninstall CoreDNS
	rm -f $(DESTDIR)$(bindir)/coredns

coredns: build/coredns/main.go
ifeq ($(GO),)
	$(error go command not found.)
endif
	$(GO) build -v -ldflags="$(LDFLAGS)" -o $@ ./$(dir $<)

coredhcp: build/coredhcp/coredhcp.go
ifeq ($(GO),)
	$(error go command not found.)
endif
	$(GO) build -v -ldflags="$(LDFLAGS)" -o $@ ./$(dir $<)

build/coredhcp/coredhcp.go: generator/coredhcp/coredhcp.go.template generator/coredhcp/plugins.txt $(rwildcard plugin/coredhcp,*.go)
ifeq ($(COREDHCP_GEN),)
	$(error coredhcp-generator command not found. It can be installed with `go install github.com/coredhcp/coredhcp/cmds/coredhcp-generator@latest`.)
endif
ifeq ($(GO),)
	$(error go command not found.)
endif
	$(COREDHCP_GEN) -t generator/coredhcp/coredhcp.go.template -f generator/coredhcp/plugins.txt $(IMPORT)/plugin/coredhcp/coresmd $(IMPORT)/plugin/coredhcp/bootloop -o $@
	$(GO) mod tidy
