#!/bin/bash

set -ax;
source /etc/bashrc
rm -rf /var/app/current/data
ln -s -T /efs /var/app/current/data
