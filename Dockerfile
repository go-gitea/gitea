FROM phusion/baseimage:0.9.19

# Use baseimage-docker's init system.
CMD ["/sbin/my_init"]

# ...put your own build instructions here...
RUN add-apt-repository -y ppa:ubuntu-lxc/lxd-stable \
  && apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y git socat libpam-runtime libpam0g-dev golang \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENV GOGS_CUSTOM /data/gogs

RUN mkdir /etc/service/gitea
COPY docker/gitea.sh /etc/service/gitea/run

RUN mkdir -p /etc/my_init.d
COPY docker/init/ /etc/my_init.d/

COPY . /app/gogs/
WORKDIR /app/gogs/
RUN ./docker/build.sh

# Clean up APT when done.
#RUN apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
