###! This dockerfile builds and starts a gitea service

FROM golang:1.15-alpine3.13 AS build-env

ARG GOPROXY="direct"
ARG TAGS="sqlite sqlite_unlock_notify bindata timetzdata"

ARG GITEA_VERSION
ARG CGO_EXTRA_CFLAGS

ARG GITEA_BUILDER_BUILD_DEPS="build-base"

# Install build dependencies
RUN apk --no-cache add $GITEA_BUILDER_BUILD_DEPS


	# build-base \
	# git \
	# nodejs \
	# npm

# Setup repo
COPY . "$GOPATH/src/code.gitea.io/gitea"

WORKDIR "$GOPATH/src/code.gitea.io/gitea"

# Checkout version if set
RUN [ -z "$GITEA_VERSION" ] || git checkout "${GITEA_VERSION}"

RUN make clean-all build

# DNM-CD(Krey): Implement automatic bumps of the alpine image
FROM alpine:3.13 AS gitea-service

LABEL maintainer="maintainers@gitea.io"

# File hierarchy
ARG GITEA_WORKDIR="/var/lib/gitea"
ARG GITEA_CUSTOMDIR="/var/lib/gitea/custom"
ARG GITEA_TEMPDIR="/var/tmp/gitea"
ARG GITEA_CONFDIR="/etc/gitea"
ARG GITEA_SRCDIR="/go/src/code.gitea.io/gitea"
ARG GITEA_APP_INI="$GITEA_CONFDIR/app.ini"
ARG GITEA_EXECUTABLE="/usr/bin/gitea"

# Permission
ARG GITEA_USER="gitea"
ARG GITEA_USER_ID="1000"
ARG GITEA_USER_HOME="/var/lib/gitea/"
ARG GITEA_USER_SHELL="/bin/nologin"
ARG GITEA_GROUP="gitea"
ARG GITEA_GROUP_ID="1000"

# Dependencies
ARG GITEA_BUILD_DEPS="make go"
ARG GITEA_RUNTIME_DEPS="git"

# Install build dependencies
RUN apk --no-cache add $GITEA_BUILD_DEPS

# Install runtime dependencies
RUN apk --no-cache add $GITEA_RUNTIME_DEPS

# Create the gitea user
## NOTE(Krey): These are busybox commands so we have to first create group and then the user added to the group
RUN true \
	# addgroup [-g GID] [-S] [USER] GROUP
	&& addgroup \
		# Create a system group
		-S \
		# Group id
		-g "$GITEA_GROUP_ID" \
		"$GITEA_GROUP" \
	# adduser [OPTIONS] USER [GROUP]
	&& adduser \
		# Create System user
		-S \
		# Don't Create home directory
		-H \
		# Don't assign a password
		-D \
		# Home directory
		-h "$GITEA_USER_HOME" \
		# Login shell
		-s "$GITEA_USER_SHELL" \
		# User id
		-u "$GITEA_USER_ID" \
		# Group
		-G "$GITEA_GROUP" \
		"$GITEA_USER"

# Copy the compiled source code in this container for installation
COPY --from=build-env "/go/src/code.gitea.io/gitea" "$GITEA_SRCDIR"

# Get gitea executable in the system
RUN cp "$GITEA_SRCDIR/gitea" "$GITEA_EXECUTABLE"

# Remove build dependencies
RUN apk del $GITEA_BUILD_DEPS

USER "$GITEA_USER"

CMD [ "$GITEA_EXECUTABLE", "--config", "$GITEA_APP_INI", "web" ]