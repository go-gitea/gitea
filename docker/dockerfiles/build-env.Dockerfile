# Dockerfile responsible of building the source code to be parsed in gitea docker environment

FROM golang:1.15-alpine3.13 AS build-env

ARG GOPROXY="direct"
ARG GITEA_VERSION
ARG TAGS="sqlite sqlite_unlock_notify bindata timetzdata"
ARG CGO_EXTRA_CFLAGS

# Build deps
RUN apk --no-cache add \
	build-base \
	git \
	nodejs \
	npm

# Setup repo
COPY . "$GOPATH/src/code.gitea.io/gitea"
WORKDIR "$GOPATH/src/code.gitea.io/gitea"

# Checkout version if set
RUN [ -z "$GITEA_VERSION" ] || git checkout "${GITEA_VERSION}"

# Install webpack-cli
RUN npm install -D webpack-cli

CMD [ "make", "clean-all", "build" ]