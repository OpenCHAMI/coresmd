FROM chainguard/wolfi-base:latest


# Include curl and tini in the final image.
RUN set -ex \
    && apk update \
    && apk add --no-cache curl tini jq dhcping \
    && rm -rf /var/cache/apk/*  \
    && rm -rf /tmp/*

# Download the latest ipxe binaries from https://github.com/OpenCHAMI/ipxe-binaries/releases and unpack them in the /tftpboot directory.
RUN set -ex \
    && mkdir -p /tftpboot \
    && latest_release_url=$(curl -s https://api.github.com/repos/OpenCHAMI/ipxe-binaries/releases/latest | jq -r '.assets[] | select(.name == "ipxe.tar.gz") | .browser_download_url') \
    && curl -L $latest_release_url -o /tmp/ipxe.tar.gz \
    && tar -xzvf /tmp/ipxe.tar.gz -C /tftpboot \
    && rm /tmp/ipxe.tar.gz


# Both coredns and coredhcp are built and addded to the same container.
# By default, coredhcp is started and coredns is not.  To start coredns, override the CMD in the 
# container runtime configuration and provide a volume with the appropriate configuration file.
COPY coredhcp /coredhcp
COPY coredns /coredns


CMD [ "/coredhcp" ]

ENTRYPOINT [ "/sbin/tini", "--" ]
