################################################################################
# STAGE 1: Generate CoreDHCP binary from sources
################################################################################

FROM golang:1.21 AS builder
ARG CGO_ENABLED=1
ARG CORESMD_IMPORT=github.com/OpenCHAMI/coresmd

RUN go install github.com/coredhcp/coredhcp/cmds/coredhcp-generator@latest

WORKDIR /coresmd
COPY go.mod go.sum ./
RUN go mod edit -replace=github.com/OpenCHAMI/coresmd=/coresmd
RUN go mod tidy
COPY . .

RUN mkdir /coredhcp
WORKDIR /coredhcp

#
# STEP 1: Generate coredhcp.go source file
#

RUN coredhcp-generator \
	-t /coresmd/generator/coredhcp.go.template \
	-f /coresmd/generator/plugins.txt \
	-o /coredhcp/cmdscoredhcp.go \
	"${CORESMD_IMPORT}"/coresmd \
	"${CORESMD_IMPORT}"/bootloop

#
# STEP 2: Build CoreDHCP
#

RUN go mod init coredhcp
RUN go mod edit -replace="${CORESMD_IMPORT}"=/coresmd
RUN go mod tidy
RUN go build -o coredhcp -ldflags "\
	-X '${CORESMD_IMPORT}/internal/version.GitCommit=$(cd /coresmd && git rev-parse HEAD)' \
	-X '${CORESMD_IMPORT}/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)' \
	-X '${CORESMD_IMPORT}/internal/version.Version=$(cd /coresmd && git describe --tags --always --abbrev=0)' \
	-X '${CORESMD_IMPORT}/internal/version.GitTag=$(cd /coresmd && git describe --tags --always --abbrev=0)' \
	-X '${CORESMD_IMPORT}/internal/version.GitState=$(cd /coresmd && if git diff-index --quiet HEAD --; then echo clean; else echo dirty; fi)' \
	-X '${CORESMD_IMPORT}/internal/version.BuildHost=$(hostname)' \
	-X '${CORESMD_IMPORT}/internal/version.GoVersion=$(go version | awk '{print $3}')' \
	-X '${CORESMD_IMPORT}/internal/version.BuildUser=$(whoami)'"

################################################################################
# STAGE 2: Copy CoreDHCP to final location
################################################################################

FROM cgr.dev/chainguard/wolfi-base

# Include curl and tini in the final image.
RUN set -ex \
    && apk update \
    && apk add --no-cache curl tini \
    && rm -rf /var/cache/apk/*  \
    && rm -rf /tmp/*

COPY --from=builder /coredhcp/coredhcp /bin/coredhcp

EXPOSE 67 67/udp

# Make dir for config file
RUN mkdir -p /etc/coredhcp
VOLUME /etc/coredhcp

CMD [ "/bin/coredhcp" ]

ENTRYPOINT [ "/sbin/tini", "--" ]
