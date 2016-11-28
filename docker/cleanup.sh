#!/bin/bash
apt-get remove -y build-essential libpam0g-dev golang software-properties-common
apt autoremove -y
apt-get clean
apt-get autoclean
rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
