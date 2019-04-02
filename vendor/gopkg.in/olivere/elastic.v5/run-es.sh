#!/bin/sh
VERSION=${VERSION:=5.6.14}
docker run --rm --privileged=true -p 9200:9200 -p 9300:9300 -v "$PWD/etc:/usr/share/elasticsearch/config" -e "bootstrap.memory_lock=true" -e "ES_JAVA_OPTS=-Xms1g -Xmx1g" docker.elastic.co/elasticsearch/elasticsearch:$VERSION elasticsearch -Expack.security.enabled=false -Expack.ml.enabled=false -Escript.inline=true -Escript.stored=true -Escript.file=true
