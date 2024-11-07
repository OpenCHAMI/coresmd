FROM chainguard/wolfi-base:latest


# Include curl and tini in the final image.
RUN set -ex \
    && apk update \
    && apk add --no-cache curl tini \
    && rm -rf /var/cache/apk/*  \
    && rm -rf /tmp/*

COPY coredhcp /coredhcp


# nobody 65534:65534
USER 65534:65534

CMD [ "/coredhcp" ]

ENTRYPOINT [ "/sbin/tini", "--" ]