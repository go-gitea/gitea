all: test install

install:
	go install

lint:
	gofmt -l -s -w . && go vet . && golint -set_exit_status=1 .

test: # The first 2 go gets are to support older Go versions
	go get github.com/arbovm/levenshtein
	go get github.com/dgryski/trifles/leven
	GO111MODULE=on go test -race -v -coverprofile=coverage.txt -covermode=atomic

bench:
	go test -run=XXX -bench=. -benchmem -count=5
