#!/bin/bash

# 停止所有运行中的容器
docker stop $(docker ps -a -q)

# 删除所有容器
docker rm $(docker ps -a -q)

# 删除所有的数据卷
# docker volume rm $(docker volume ls -q)
