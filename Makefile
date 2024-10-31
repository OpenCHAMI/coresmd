BIN                ?= coredhcp
GO                 ?= go
GOPATH             ?= $(shell $(GO) env GOPATH)
COREDHCP_IMPORT    ?= github.com/coredhcp/coredhcp
COREDHCP_GENERATOR ?= $(GOPATH)/bin/coredhcp-generator
CORESMD_IMPORT     ?= github.com/OpenCHAMI/coresmd

GIT_COMMIT = $(shell git rev-parse HEAD)
BUILD_TIME = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION    = $(shell git describe --tags --always --abbrev=0)
GIT_TAG    = $(shell git describe --tags --always --abbrev=0)
GIT_STATE  = $(shell if git diff-index --quiet HEAD --; then echo clean; else echo dirty; fi)
BUILD_HOST = $(shell hostname)
GO_VERSION = $(shell go version | awk '{print $3}')
BUILD_USER = $(shell whoami)

all: $(BIN)

$(COREDHCP_GENERATOR):
	GOPATH=$(GOPATH) $(GO) install $(GOREDHCP_IMPORT)/cmds/coredhcp-generator

cmd/coredhcp.go: $(COREDHCP_GENERATOR) generator/coredhcp.go.template generator/plugins.txt
	$(COREDHCP_GENERATOR) \
		-t generator/coredhcp.go.template \
		-f generator/plugins.txt \
		-o $@ \
		$(CORESMD_IMPORT)/coresmd \
		$(CORESMD_IMPORT)/bootloop

$(BIN): cmd/coredhcp.go
	$(GO) build -o $@ -ldflags " -s -w \
		-X '$(CORESMD_IMPORT)/internal/version.GitCommit=$(GIT_COMMIT)' \
		-X '$(CORESMD_IMPORT)/internal/version.BuildTime=$(BUILD_TIME)' \
		-X '$(CORESMD_IMPORT)/internal/version.Version=$(VERSION)' \
		-X '$(CORESMD_IMPORT)/internal/version.GitTag=$(GIT_TAG)' \
		-X '$(CORESMD_IMPORT)/internal/version.GitState=$(GIT_STATE)' \
		-X '$(CORESMD_IMPORT)/internal/version.BuildHost=$(BUILD_HOST)' \
		-X '$(CORESMD_IMPORT)/internal/version.GoVersion=$(GO_VERSION)' \
		-X '$(CORESMD_IMPORT)/internal/version.BuildUser=$(BUILD_USER)'" \
		$^

clean:
	$(GO) clean -i -x ./cmd
	rm -f coredhcp
	rm -f cmd/coredhcp.go
