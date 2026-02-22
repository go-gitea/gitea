# syntax=docker/dockerfile:1
# Build frontend on the native platform to avoid QEMU-related issues with esbuild/webpack
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.26-alpine3.23 AS frontend-build
RUN apk --no-cache add build-base git nodejs pnpm
WORKDIR /src
COPY --exclude=.git/ . .
RUN --mount=type=cache,target=/root/.local/share/pnpm/store make frontend

# Build backend for each target platform
FROM docker.io/library/golang:1.26-alpine3.23 AS build-env

ARG GOPROXY=direct

ARG GITEA_VERSION
ARG TAGS="sqlite sqlite_unlock_notify"
ENV TAGS="bindata timetzdata $TAGS"
ARG CGO_EXTRA_CFLAGS

# Build deps
RUN apk --no-cache add \
    build-base \
    git

WORKDIR ${GOPATH}/src/code.gitea.io/gitea
# Use COPY instead of bind mount as read-only one breaks makefile state tracking and read-write one needs binary to be moved as it's discarded.
# ".git" directory is mounted separately later only for version data extraction.
COPY --exclude=.git/ . .
COPY --from=frontend-build /src/public/assets public/assets

# Build gitea, .git mount is required for version data
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target="/root/.cache/go-build" \
    --mount=type=bind,source=".git/",target=".git/" \
    make backend

COPY docker/root /tmp/local

# Set permissions for builds that made under windows which strips the executable bit from file
RUN chmod 755 /tmp/local/usr/bin/entrypoint \
              /tmp/local/usr/local/bin/* \
              /tmp/local/etc/s6/gitea/* \
              /tmp/local/etc/s6/openssh/* \
              /tmp/local/etc/s6/.s6-svscan/* \
              /go/src/code.gitea.io/gitea/gitea

FROM docker.io/library/alpine:3.23 AS gitea

EXPOSE 22 3000

RUN apk --no-cache add \
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

COPY --from=build-env /tmp/local /
COPY --from=build-env /go/src/code.gitea.io/gitea/gitea /app/gitea/gitea

ENV USER=git
ENV GITEA_CUSTOM=/data/gitea

VOLUME ["/data"]

# HINT: HEALTH-CHECK-ENDPOINT: don't use HEALTHCHECK, search this hint keyword for more information
ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/usr/bin/s6-svscan", "/etc/s6"]
