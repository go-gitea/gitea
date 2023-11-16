# Build stage
FROM docker.io/library/node:20-alpine3.18 AS build-frontend

# Build deps
RUN apk --no-cache add build-base git \
  && rm -rf /var/cache/apk/*

# Setup repo
WORKDIR /usr/src/code.gitea.io/gitea

COPY Makefile .

# Download NPM Packages
COPY package.json .
COPY package-lock.json .

RUN make deps-frontend

# Copy source files
COPY ./webpack.config.js .
COPY ./assets ./assets
COPY ./public ./public
COPY ./web_src ./web_src

ARG GITHUB_REF_NAME
ARG GITHUB_REF_TYPE
ARG DOCKER_GITEA_VERSION

ENV GITHUB_REF_NAME=${GITHUB_REF_NAME:-docker-develop}
ENV GITHUB_REF_TYPE=${GITHUB_REF_TYPE:-branch}
ENV DOCKER_GITEA_VERSION=${DOCKER_GITEA_VERSION:-${GITHUB_REF_NAME}}

# Build frontend
RUN make clean-all frontend

# Build stage
FROM docker.io/library/golang:1.21-alpine3.18 AS build-backend

ARG GOPROXY
ENV GOPROXY ${GOPROXY:-direct}

ARG TAGS="sqlite sqlite_unlock_notify"
ENV TAGS "bindata timetzdata $TAGS"
ARG CGO_EXTRA_CFLAGS

# Build deps
RUN apk --no-cache add build-base git \
  && rm -rf /var/cache/apk/*

# Setup repo
WORKDIR ${GOPATH}/src/code.gitea.io/gitea

COPY Makefile .

# Download Golang Modules
COPY go.mod .
COPY go.sum .

RUN make deps-backend

# Copy source files
COPY ./build ./build
COPY ./cmd ./cmd
COPY ./models ./models
COPY ./modules ./modules
COPY ./options ./options
COPY ./routers ./routers
COPY ./services ./services
COPY ./templates ./templates
COPY ./build.go .
COPY ./main.go .

# Clean directory
RUN make clean-all

# Copy frontend build artifacts
COPY --from=build-frontend /usr/src/code.gitea.io/gitea/public ./public

ARG GITHUB_REF_NAME
ARG GITHUB_REF_TYPE
ARG DOCKER_GITEA_VERSION

ENV GITHUB_REF_NAME=${GITHUB_REF_NAME:-docker-develop}
ENV GITHUB_REF_TYPE=${GITHUB_REF_TYPE:-branch}
ENV DOCKER_GITEA_VERSION=${DOCKER_GITEA_VERSION:-${GITHUB_REF_NAME}}

# Build backend
RUN make backend

# Begin env-to-ini build
COPY contrib/environment-to-ini/environment-to-ini.go contrib/environment-to-ini/environment-to-ini.go
COPY ./custom ./custom

RUN go build contrib/environment-to-ini/environment-to-ini.go

FROM docker.io/library/alpine:3.18 AS gitea-base
LABEL maintainer="maintainers@gitea.io"

RUN apk --no-cache add \
  bash \
  ca-certificates \
  gettext \
  git \
  curl \
  gnupg \
  && rm -rf /var/cache/apk/*

RUN addgroup -S -g 1000 git

COPY --chmod=644 ./contrib/autocompletion/bash_autocomplete /etc/profile.d/gitea_bash_autocomplete.sh
COPY --chmod=755 --from=build-backend /go/src/code.gitea.io/gitea/gitea /app/gitea/gitea
COPY --chmod=755 --from=build-backend /go/src/code.gitea.io/gitea/environment-to-ini /usr/local/bin/environment-to-ini

FROM gitea-base AS gitea-rootless

EXPOSE 2222 3000

RUN apk --no-cache add \
  dumb-init \
  && rm -rf /var/cache/apk/*

RUN adduser \
  -S -H -D \
  -h /var/lib/gitea/git \
  -s /bin/bash \
  -u 1000 \
  -G git \
  git

RUN mkdir -p /var/lib/gitea /etc/gitea
RUN chown git:git /var/lib/gitea /etc/gitea

# Copy local files
COPY --chmod=755 docker/rootless /

# git:git
USER 1000:1000
ENV GITEA_WORK_DIR /var/lib/gitea
ENV GITEA_CUSTOM /var/lib/gitea/custom
ENV GITEA_TEMP /tmp/gitea
ENV TMPDIR /tmp/gitea

# TODO add to docs the ability to define the ini to load (useful to test and revert a config)
ENV GITEA_APP_INI /etc/gitea/app.ini
ENV HOME "/var/lib/gitea/git"
VOLUME ["/var/lib/gitea", "/etc/gitea"]
WORKDIR /var/lib/gitea

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/usr/local/bin/docker-entrypoint.sh"]
CMD []

FROM gitea-base AS gitea

EXPOSE 22 3000

RUN apk --no-cache add \
  linux-pam \
  openssh \
  s6 \
  sqlite \
  su-exec \
  && rm -rf /var/cache/apk/*

RUN adduser \
  -S -H -D \
  -h /data/git \
  -s /bin/bash \
  -u 1000 \
  -G git \
  git && \
  echo "git:*" | chpasswd -e

ENV USER git
ENV GITEA_CUSTOM /data/gitea

VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/bin/s6-svscan", "/etc/s6"]

COPY --chmod=755 docker/root /
