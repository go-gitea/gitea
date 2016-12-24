default: check

check:
	go test && go test -compiler gccgo

docs:
	godoc2md github.com/juju/errors > README.md
	sed -i 's|\[godoc-link-here\]|[![GoDoc](https://godoc.org/github.com/juju/errors?status.svg)](https://godoc.org/github.com/juju/errors)|' README.md 


.PHONY: default check docs
