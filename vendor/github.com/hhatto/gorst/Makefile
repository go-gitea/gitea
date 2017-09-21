all:
	@echo 'targets: cmd test nuke parser clean'

cmd:
	go build -v -o gorst cmd/gorst/main.go

test:
	go test -v

package: parser.leg.go
	go install -v

clean:
	go clean . ./...
	rm -rf ,,prevmd ,,pmd
	rm -f *.html gorst
	
parser:	parser.leg.go

nuke:
	rm -f parser.leg.go

install-packages:
	go get -v -u golang.org/x/tools/cmd/goimports

# LEG parser rules
#
ifeq ($(MAKECMDGOALS),parser)
include $(shell go list -f '{{.Dir}}' github.com/knieriem/peg)/Make.inc
%.leg.go: %.leg $(LEG)
	$(LEG) -verbose -switch -O all $< > $@
	goimports -w parser.leg.go
	go fmt parser.leg.go
endif


include misc/devel.mk

.PHONY: \
	all\
	cmd\
	nuke\
	test\
	package\
	parser\
