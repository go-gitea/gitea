###################################
#Build stage
FROM gitea/gitea-base AS build-env

ARG TAGS="sqlite"
ENV TAGS "bindata netgo $TAGS"
ENV GOPATH /go/
ENV PATH "${PATH}:${GOPATH}/bin"

RUN apk --no-cache add go git build-base

ADD . ${GOPATH}/src/code.gitea.io/gitea
WORKDIR ${GOPATH}/src/code.gitea.io/gitea

ARG GITEA_VERSION
RUN if [ -n "${GITEA_VERSION}" ]; then git checkout ${GITEA_VERSION}; fi \
 && make clean generate build

###################################
#Run stage
FROM gitea/gitea-base
LABEL maintainer "Thomas Boerger <thomas@webhippie.de>"

ARG UID=1000
ARG GID=1000

EXPOSE 22 3000

RUN apk --no-cache add \
    su-exec ca-certificates sqlite bash git linux-pam s6 curl openssh tzdata \
 && addgroup -S -g ${GID} git \
 && adduser -S -H -D -h /data/git -s /bin/bash -u ${UID} -G git git \
 && echo "git:$(dd if=/dev/urandom bs=24 count=1 | base64)" | chpasswd

ENV USER git
ENV GITEA_CUSTOM /data/gitea

VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/bin/s6-svscan", "/etc/s6"]

COPY docker /
COPY --from=build-env /go/src/code.gitea.io/gitea/gitea /app/gitea/gitea
