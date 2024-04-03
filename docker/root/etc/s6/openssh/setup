#!/bin/bash

if [ ! -d /data/ssh ]; then
    mkdir -p /data/ssh
fi

if [ ! -f /data/ssh/ssh_host_ed25519_key ]; then
    echo "Generating /data/ssh/ssh_host_ed25519_key..."
    ssh-keygen -t ed25519 -f /data/ssh/ssh_host_ed25519_key -N "" > /dev/null
fi

if [ ! -f /data/ssh/ssh_host_rsa_key ]; then
    echo "Generating /data/ssh/ssh_host_rsa_key..."
    ssh-keygen -t rsa -b 3072 -f /data/ssh/ssh_host_rsa_key -N "" > /dev/null
fi

if [ ! -f /data/ssh/ssh_host_ecdsa_key ]; then
    echo "Generating /data/ssh/ssh_host_ecdsa_key..."
    ssh-keygen -t ecdsa -b 256 -f /data/ssh/ssh_host_ecdsa_key -N "" > /dev/null
fi

if [ -e /data/ssh/ssh_host_ed25519_cert ]; then
  SSH_ED25519_CERT=${SSH_ED25519_CERT:-"/data/ssh/ssh_host_ed25519_cert"}
fi

if [ -e /data/ssh/ssh_host_rsa_cert ]; then
  SSH_RSA_CERT=${SSH_RSA_CERT:-"/data/ssh/ssh_host_rsa_cert"}
fi

if [ -e /data/ssh/ssh_host_ecdsa_cert ]; then
  SSH_ECDSA_CERT=${SSH_ECDSA_CERT:-"/data/ssh/ssh_host_ecdsa_cert"}
fi

if [ -d /etc/ssh ]; then
    SSH_PORT=${SSH_PORT:-"22"} \
    SSH_LISTEN_PORT=${SSH_LISTEN_PORT:-"${SSH_PORT}"} \
    SSH_ED25519_CERT="${SSH_ED25519_CERT:+"HostCertificate "}${SSH_ED25519_CERT}" \
    SSH_RSA_CERT="${SSH_RSA_CERT:+"HostCertificate "}${SSH_RSA_CERT}" \
    SSH_ECDSA_CERT="${SSH_ECDSA_CERT:+"HostCertificate "}${SSH_ECDSA_CERT}" \
    SSH_MAX_STARTUPS="${SSH_MAX_STARTUPS:+"MaxStartups "}${SSH_MAX_STARTUPS}" \
    SSH_MAX_SESSIONS="${SSH_MAX_SESSIONS:+"MaxSessions "}${SSH_MAX_SESSIONS}" \
    SSH_INCLUDE_FILE="${SSH_INCLUDE_FILE:+"Include "}${SSH_INCLUDE_FILE}" \
    SSH_LOG_LEVEL=${SSH_LOG_LEVEL:-"INFO"} \
    envsubst < /etc/templates/sshd_config > /etc/ssh/sshd_config

    chmod 0644 /etc/ssh/sshd_config
fi

chown root:root /data/ssh/*
chmod 0700 /data/ssh
chmod 0600 /data/ssh/*
