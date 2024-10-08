################################################################################
# STAGE 1: Build CoreDHCP
################################################################################

FROM golang:1.21 AS builder
ARG CGO_ENABLED=0

#
# STEP 1: Clone coredhcp and build coredhcp-generator
#

RUN git clone https://github.com/coredhcp/coredhcp /coredhcp
WORKDIR /coredhcp

RUN go mod download
RUN go build ./cmds/coredhcp-generator

#
# STEP 2: Copy source tree and generate CoreDHCP main.go
#

WORKDIR /coresmd
COPY go.mod go.sum ./
RUN go mod edit -replace=github.com/synackd/coresmd=/coresmd
RUN go mod download
COPY . .

RUN mkdir /coredhcp-coresmd
WORKDIR /coredhcp-coresmd

RUN /coredhcp/coredhcp-generator \
	-t /coredhcp/cmds/coredhcp-generator/coredhcp.go.template \
	-f /coredhcp/cmds/coredhcp-generator/core-plugins.txt \
	-o /coredhcp-coresmd/coredhcp.go \
	github.com/synackd/coresmd

#
# STEP 3: Build CoreDHCP
#

RUN go mod init t.t/coredhcp
RUN go mod edit -replace=github.com/coredhcp/coredhcp=/coredhcp
RUN go mod edit -replace=github.com/synackd/coresmd=/coresmd
RUN go mod tidy
RUN go build -o coredhcp

################################################################################
# STAGE 2: Copy CoreDHCP to final location
################################################################################

FROM cgr.dev/chainguard/wolfi-base

#RUN apk add --no-cache tini

COPY --from=builder /coredhcp-coresmd/coredhcp /bin/coredhcp

EXPOSE 67 67/udp

# Make dir for config file
RUN mkdir -p /etc/coredhcp
VOLUME /etc/coredhcp

ENTRYPOINT [ "/bin/coredhcp" ]
