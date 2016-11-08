FROM alpine:3.3
MAINTAINER jp@roemer.im

# Install system utils & Gogs runtime dependencies
ADD https://github.com/tianon/gosu/releases/download/1.9/gosu-amd64 /usr/sbin/gosu
RUN chmod +x /usr/sbin/gosu \
 && apk --no-cache --no-progress add ca-certificates bash git linux-pam s6 curl openssh socat tzdata

ENV GITEA_CUSTOM /data/gogs

COPY . /app/gogs/
WORKDIR /app/gogs/
RUN ./docker/build.sh

# Configure LibC Name Service
COPY docker/nsswitch.conf /etc/nsswitch.conf

# Configure Docker Container
VOLUME ["/data"]
EXPOSE 22 3000
ENTRYPOINT ["docker/start.sh"]
CMD ["/bin/s6-svscan", "/app/gogs/docker/s6/"]
