.PHONY: chromad upload all

all: README.md tokentype_string.go

README.md: lexers/*/*.go
	./table.py

tokentype_string.go: types.go
	go generate

chromad:
	(cd ./cmd/chromad && go get github.com/GeertJohan/go.rice/rice@master && go install github.com/GeertJohan/go.rice/rice)
	rm -f chromad
	(export CGOENABLED=0 GOOS=linux ; cd ./cmd/chromad && go build -o ../../chromad .)
	rice append -i ./cmd/chromad --exec=./chromad

upload: chromad
	scp chromad root@swapoff.org: && \
		ssh root@swapoff.org 'install -m755 ./chromad /srv/http/swapoff.org/bin && service chromad restart'
