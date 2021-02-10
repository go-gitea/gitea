###! This dockerfile builds and starts a gitea service

FROM golang:1.15-alpine3.13 AS build-env

ARG GOPROXY="direct"

# DNM-INVESTIGATE(Krey): Investigate these tags and optimize their usage
ARG TAGS="sqlite sqlite_unlock_notify bindata timetzdata"

ARG GITEA_VERSION
ARG CGO_EXTRA_CFLAGS

ARG GITEA_BUILDER_BUILD_DEPS="build-base nodejs npm git"

# Install build dependencies
RUN apk --no-cache add $GITEA_BUILDER_BUILD_DEPS

# Setup repo
COPY . "$GOPATH/src/code.gitea.io/gitea"

WORKDIR "$GOPATH/src/code.gitea.io/gitea"

# Checkout version if set
RUN [ -z "$GITEA_VERSION" ] || { make clean-all && git checkout "${GITEA_VERSION}" ;}

# Build the source code
RUN make clean-all build

# DNM-CD(Krey): Implement automatic bumps of the alpine image
FROM alpine:3.13 AS gitea-service

LABEL maintainer="maintainers@gitea.io"

# File hierarchy
## NOTE-DUP_CODE(Krey): Changes of these values has to be updated in `docker/wrapper/gitea.sh` as well
ARG GITEA_WORKDIR="/srv/gitea"
RUN mkdir -p "$GITEA_WORKDIR"

ARG GITEA_CUSTOMDIR="$GITEA_WORKDIR/custom"
RUN mkdir -p "$GITEA_CUSTOMDIR"

ARG GITEA_TEMPDIR="/var/tmp/gitea"
RUN mkdir -p "$GITEA_TEMPDIR"

ARG GITEA_CONFDIR="$GITEA_CUSTOMDIR/conf"
RUN mkdir -p "$GITEA_CONFDIR"

ARG GITEA_SRCDIR="/go/src/code.gitea.io/gitea"
ARG GITEA_APP_INI="$GITEA_CONFDIR/app.ini"

RUN mkdir -p "$GITEA_WORKDIR"

ARG GITEA_EXECUTABLE="$GITEA_WORKDIR/gitea"

# Permission
ARG GITEA_USER="gitea"
ARG GITEA_USER_ID="1000"
ARG GITEA_USER_HOME="$GITEA_WORKDIR"
ARG GITEA_USER_SHELL="/bin/nologin"
ARG GITEA_GROUP="gitea"
ARG GITEA_GROUP_ID="1000"

# Dependencies
ARG GITEA_RUNTIME_DEPS="git"

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

ARG GITEA_WRAPPER_SCRIPT="$GITEA_WORKDIR/gitea-wrapper"
COPY docker/wrapper/gitea.sh "$GITEA_WRAPPER_SCRIPT"

RUN chown -R "$GITEA_USER:$GITEA_GROUP" "$GITEA_USER_HOME"

USER "$GITEA_USER"

# FIXME(Krey): Expecting '$GITEA_EXECUTABLE/$GITEA_WRAPPER_SCRIPT' to expand here, but it is not available at the CMD scope
CMD [ "/srv/gitea/gitea", "web" ]