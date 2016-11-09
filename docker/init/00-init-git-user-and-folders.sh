#!/bin/bash
export USER_ID=$(id -u)
export GROUP_ID=$(id -g)

if [[ ${USER_ID} == 0 ]] ; then
    # dont let git be user of root
    export USER_ID=1000
fi

if [[ ${GROUP_ID} == 0 ]] ; then
    # git should not be in the same group as root
    export GROUP_ID=1000
    addgroup --gid ${GROUP_ID} git
fi

# Create git user
useradd --home-dir /data/git --shell /bin/bash --no-create-home --no-user-group --gid ${GROUP_ID} --uid ${USER_ID} git && passwd -u git

# Create VOLUME subfolder
for f in /data/gogs/data /data/gogs/conf /data/gogs/log /data/git /data/ssh; do
	if ! test -d $f; then
		mkdir -p $f
	fi
done

if ! test -d ~git/.ssh; then
    mkdir -p ~git/.ssh
    chmod 700 ~git/.ssh
fi

if ! test -f ~git/.ssh/environment; then
    echo "GOGS_CUSTOM=${GOGS_CUSTOM}" > ~git/.ssh/environment
    chmod 600 ~git/.ssh/environment
fi

# Link volumed data with app data
ln -sf /data/gogs/log  ./log
ln -sf /data/gogs/data ./data

# Backward Compatibility with Gogs Container v0.6.15
ln -sf /data/git /home/git

chown -R git:git /data /app/gogs ~git/
chmod 0755 /data /data/gogs ~git/

echo "export GOGS_CUSTOM=${GOGS_CUSTOM}" >> /etc/profile
