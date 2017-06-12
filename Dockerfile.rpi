FROM multiarch/alpine:armhf-v3.5
MAINTAINER Thomas Boerger <thomas@webhippie.de>

EXPOSE 22 3000

RUN apk --no-cache add \
    su-exec \
    ca-certificates \
    sqlite \
    bash \
    git \
    linux-pam \
    s6 \
    curl \
    openssh \
    tzdata
RUN addgroup \
    -S -g 1000 \
    git && \
  adduser \
    -S -H -D \
    -h /data/git \
    -s /bin/bash \
    -u 1000 \
    -G git \
    git && \
  echo "git:$(date +%s | sha256sum | base64 | head -c 32)" | chpasswd

ENV USER git
ENV GITEA_CUSTOM /data/gitea
ENV GODEBUG=netdns=go

VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/bin/s6-svscan", "/etc/s6"]

COPY docker /
COPY gitea /app/gitea/gitea
