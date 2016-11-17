FROM willemvd/ubuntu-unprivileged-git-ssh:latest

USER root

COPY . /app/gitea/
WORKDIR /app/gitea/

# remove when using pre-build gitea
RUN docker/prepare.sh && docker/build.sh && docker/cleanup.sh
# end remove

RUN docker/init/00-init-git-user-and-folders.sh && docker/init/10-setup-gitea.sh

USER git

# persistent volume for the host ssh key and gitea data
VOLUME ["/etc/ssh/keys", "/data"]

EXPOSE 2222 3000

# Use baseimage-docker's init system.
ENTRYPOINT ["/sbin/my_init", "--"]
