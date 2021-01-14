.PHONY: test fuzz

test:
	go test -coverprofile=profile.out -coverpkg=github.com/yuin/goldmark,github.com/yuin/goldmark/ast,github.com/yuin/goldmark/extension,github.com/yuin/goldmark/extension/ast,github.com/yuin/goldmark/parser,github.com/yuin/goldmark/renderer,github.com/yuin/goldmark/renderer/html,github.com/yuin/goldmark/text,github.com/yuin/goldmark/util ./...

cov: test
	go tool cover -html=profile.out

fuzz:
	which go-fuzz > /dev/null 2>&1 || (GO111MODULE=off go get -u github.com/dvyukov/go-fuzz/go-fuzz github.com/dvyukov/go-fuzz/go-fuzz-build; GO111MODULE=off go get -d github.com/dvyukov/go-fuzz-corpus; true)
	rm -rf ./fuzz/corpus
	rm -rf ./fuzz/crashers
	rm -rf ./fuzz/suppressions
	rm -f ./fuzz/fuzz-fuzz.zip
	cd ./fuzz && go-fuzz-build
	cd ./fuzz && go-fuzz
