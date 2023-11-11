# Build stage
FROM docker.io/library/golang:1.21-alpine3.18 AS build-env

ARG GOPROXY
ENV GOPROXY ${GOPROXY:-direct}

ARG GITEA_VERSION
ARG TAGS="sqlite sqlite_unlock_notify"
ENV TAGS "bindata timetzdata $TAGS"
ARG CGO_EXTRA_CFLAGS

# Build deps
RUN apk --no-cache add \
    build-base \
    git \
    nodejs \
    npm \
    && rm -rf /var/cache/apk/*

# Setup repo
WORKDIR ${GOPATH}/src/code.gitea.io/gitea

COPY ./go.mod .

RUN go mod download

COPY package.json .
COPY package-lock.json .

RUN npm install --no-save --verbose

COPY . .

# Checkout version if set
RUN if [ -n "${GITEA_VERSION}" ]; then git checkout "${GITEA_VERSION}"; fi \
 && make clean-all build

# Begin env-to-ini build
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

FROM gitea-base AS gitea-rootless
LABEL maintainer="maintainers@gitea.io"

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
COPY --chmod=755 docker/rootless /tmp/local

COPY --from=build-env --chmod=755 --chown=root:root /go/src/code.gitea.io/gitea/gitea /app/gitea/gitea
COPY --from=build-env --chmod=755 --chown=root:root /go/src/code.gitea.io/gitea/environment-to-ini /usr/local/bin/environment-to-ini
COPY --from=build-env --chmod=644 /go/src/code.gitea.io/gitea/contrib/autocompletion/bash_autocomplete /etc/profile.d/gitea_bash_autocomplete.sh

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
LABEL maintainer="maintainers@gitea.io"

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

COPY --chmod=755 docker/root /tmp/local

COPY --from=build-env --chmod=755 /go/src/code.gitea.io/gitea/gitea /app/gitea/gitea
COPY --from=build-env --chmod=755 /go/src/code.gitea.io/gitea/environment-to-ini /usr/local/bin/environment-to-ini
COPY --from=build-env --chmod=644 /go/src/code.gitea.io/gitea/contrib/autocompletion/bash_autocomplete /etc/profile.d/gitea_bash_autocomplete.sh
