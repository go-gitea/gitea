#!/bin/bash

find . -type f -name '*.go' -a ! -name traceinit.go -exec grep -l '^func init()' {} "+" | while read file
do

	sed -i -r '0,/^[ \t]*package/ s-^([ \t]*package.*)$-\1\nimport "code.gitea.io/gitea/traceinit"-;
0,/^func init\(\)/ s-^(func init\(\).*)$-\1\ntraceinit.Trace("'"$file"'")-' "$file"

done
