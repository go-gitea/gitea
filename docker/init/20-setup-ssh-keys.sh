#!/bin/bash

# Check if host keys are present, else create them
if ! test -f /data/ssh/ssh_host_rsa_key; then
    ssh-keygen -q -f /data/ssh/ssh_host_rsa_key -N '' -t rsa
fi

if ! test -f /data/ssh/ssh_host_dsa_key; then
    ssh-keygen -q -f /data/ssh/ssh_host_dsa_key -N '' -t dsa
fi

if ! test -f /data/ssh/ssh_host_ecdsa_key; then
    ssh-keygen -q -f /data/ssh/ssh_host_ecdsa_key -N '' -t ecdsa
fi

if ! test -f /data/ssh/ssh_host_ed25519_key; then
    ssh-keygen -q -f /data/ssh/ssh_host_ed25519_key -N '' -t ed25519
fi

cp /data/ssh/* /etc/ssh/

# Set correct right to ssh keys
#chown -R root:root /data/ssh/*
#chmod 0700 /data/ssh
#chmod 0600 /data/ssh/*

