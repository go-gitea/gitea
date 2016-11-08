FROM alpine:3.4

MAINTAINER Rachid Zarouali <xinity77@gmail.com>

# Install system utils & gitea runtime dependencies
ADD https://github.com/tianon/gosu/releases/download/1.9/gosu-amd64 /usr/sbin/gosu
RUN chmod +x /usr/sbin/gosu \
 && apk --no-cache --no-progress add ca-certificates bash git linux-pam s6 curl openssh socat tzdata

ENV GITEA_CUSTOM /data/gitea

COPY . /app/gitea/
WORKDIR /app/gitea/

# Configure LibC Name Service
COPY docker/nsswitch.conf /etc/nsswitch.conf

# creating data folder tree
RUN mkdir -p /data/gitea/data /data/gitea/conf /data/gitea/log /data/git /data/ssh

# Configure Docker Container
VOLUME ["/data"]
EXPOSE 22 3000
ENTRYPOINT ["docker/start.sh"]
CMD ["/bin/s6-svscan", "/app/gitea/docker/s6/"]
