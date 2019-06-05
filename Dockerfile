
###################################
#Build stage
FROM golang:1.12-alpine3.9 AS build-env

ARG GITEA_VERSION
ARG TAGS="sqlite sqlite_unlock_notify"
ENV TAGS "bindata $TAGS"

#Build deps
RUN apk --no-cache add build-base git

#Setup repo
COPY . ${GOPATH}/src/code.gitea.io/gitea
WORKDIR ${GOPATH}/src/code.gitea.io/gitea

#Checkout version if set
RUN if [ -n "${GITEA_VERSION}" ]; then git checkout "${GITEA_VERSION}"; fi \
 && make clean generate build

FROM alpine:3.9
LABEL maintainer="maintainers@gitea.io"

EXPOSE 2222 3000

RUN apk --no-cache add \
    bash \
    ca-certificates \
    curl \
    gettext \
    git \
    openssh-keygen \
    sqlite \
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
  echo "git:$(dd if=/dev/urandom bs=24 count=1 status=none | base64)" | chpasswd


ENV HOME "/data/git"
RUN mkdir -p /data /app/gitea && chmod 777 /data
RUN ln -s /app/gitea/gitea /usr/local/bin/gitea

USER git:git
ENV GITEA_CUSTOM /data/gitea

VOLUME ["/data"]

WORKDIR /app/gitea
ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/app/gitea/gitea", "web"]

COPY docker/root /
COPY --from=build-env /go/src/code.gitea.io/gitea/gitea /app/gitea/gitea
