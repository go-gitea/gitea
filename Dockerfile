# syntax=docker/dockerfile:1
# Build stage
FROM docker.io/library/golang:1.25-alpine3.22 AS build-env

ARG GOPROXY=direct

ARG GITEA_VERSION
ARG TAGS="sqlite sqlite_unlock_notify"
ENV TAGS="bindata timetzdata $TAGS"
ARG CGO_EXTRA_CFLAGS

# Build deps
RUN apk --no-cache add \
    build-base \
    git \
    nodejs \
    pnpm

WORKDIR ${GOPATH}/src/code.gitea.io/gitea
COPY --exclude=.git/ . .

# Checkout version if set
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target="/root/.cache/go-build" \
    --mount=type=cache,target=/root/.local/share/pnpm/store \
    --mount=type=bind,source=".git/",target=".git/" \
    if [ -n "${GITEA_VERSION}" ]; then git checkout "${GITEA_VERSION}"; fi \
    && make

# Begin env-to-ini build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target="/root/.cache/go-build" \
    go build contrib/environment-to-ini/environment-to-ini.go

FROM docker.io/library/alpine:3.22 AS gitea

EXPOSE 22 3000

RUN apk add --no-cache \
    bash \
    ca-certificates \
    curl \
    gettext \
    git \
    linux-pam \
    openssh \
    s6 \
    sqlite \
    su-exec \
    gnupg

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
  echo "git:*" | chpasswd -e

COPY docker/root /
COPY --chmod=755 --from=build-env /go/src/code.gitea.io/gitea/gitea /app/gitea/gitea
COPY --chmod=755 --from=build-env /go/src/code.gitea.io/gitea/environment-to-ini /usr/local/bin/environment-to-ini

ENV USER=git
ENV GITEA_CUSTOM=/data/gitea

VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/usr/bin/s6-svscan", "/etc/s6"]
