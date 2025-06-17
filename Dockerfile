
###################################
#Build stage
FROM golang:1.24.2-alpine3.21 AS build-env

#ARG GOPROXY
#ENV GOPROXY ${GOPROXY:-direct}

ARG GITEA_VERSION="release/v1.12"
ARG TAGS="sqlite sqlite_unlock_notify"
ENV TAGS "bindata $TAGS"
ENV NODE_OPTIONS "--openssl-legacy-provider"
#ARG CGO_EXTRA_CFLAGS

#Build deps
RUN apk update && \
    apk upgrade
RUN apk --no-cache add build-base git nodejs npm
RUN apk --no-cache add sqlite>3.38

#Setup repo
COPY . ${GOPATH}/src/code.gitea.io/gitea
WORKDIR ${GOPATH}/src/code.gitea.io/gitea

#Checkout version if set
RUN if [ -n "${GITEA_VERSION}" ]; then git checkout "${GITEA_VERSION}"; fi \
 #&& make build
 && make clean-all build

FROM alpine:3.22
LABEL maintainer="maintainers@gitea.io"

EXPOSE 22 3000

RUN apk --no-cache add bash
RUN apk --no-cache add ca-certificates
RUN apk --no-cache add curl
RUN apk del libidn2
RUN apk --no-cache add libidn2
RUN apk --no-cache add gettext
#RUN apk --no-cache add gettext
RUN apk --no-cache add git
RUN apk --no-cache add linux-pam
#RUN apk --no-cache add openssh
RUN apk --no-cache add openssh
RUN apk --no-cache add s6
RUN apk --no-cache add sqlite
#RUN apk --no-cache add sqlite
RUN apk --no-cache add su-exec
RUN apk --no-cache add tzdata
RUN apk --no-cache add nettle
RUN apk --no-cache add gnupg

RUN apk update && \
    apk upgrade


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

ENV USER git
ENV GITEA_CUSTOM /data/gitea

VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/bin/s6-svscan", "/etc/s6"]

#COPY docker/root /
COPY --from=build-env /go/src/code.gitea.io/gitea/gitea /app/gitea/gitea
RUN ln -s /app/gitea/gitea /usr/local/bin/gitea
