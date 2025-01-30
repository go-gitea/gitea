#!/bin/bash

# 检查传入的参数数量
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <GITEA_RUNNER_REGISTRATION_TOKEN>"
    exit 1
fi

token=$1

docker stop gitea_runner
docker rm gitea_runner
docker run \
    -v $PWD/docker_compose_conf/act_runner/config.yaml:/config.yaml \
    -v $PWD/docker_compose_conf/act_runner/data:/data \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -e CONFIG_FILE=/config.yaml \
	  -e GITEA_INSTANCE_URL=http://gitea:3000 \
    -e GITEA_RUNNER_REGISTRATION_TOKEN=$token \
    -e GITEA_RUNNER_NAME=runner-main \
    -e GITEA_RUNNER_LABELS=main \
    --name gitea_runner \
    --network gitea \
    -d gitea/act_runner:latest
