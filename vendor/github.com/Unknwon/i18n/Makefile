.PHONY: build test bench vet

build: vet bench

test:
	go test -v -cover

bench:
	go test -v -cover -test.bench=. -test.benchmem

vet:
	go vet