#!/bin/bash

find . -type f -name '*.go' -exec grep -l '^func init()' {} "+" | while read file
do

    gsed -i -r '0,/^[ \t]*package/ s-^([ \t]*package.*)$-\1\nimport "code.gitea.io/gitea/traceinit"-;
0,/^func init\(\)/ s-^(func init\(\).*)$-\1\ntraceinit.trace("'"$file"'")-' "$file"
    exit

done
