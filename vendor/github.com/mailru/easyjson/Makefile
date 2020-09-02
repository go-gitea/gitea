all: test

clean:
	rm -rf bin
	rm -rf tests/*_easyjson.go
	rm -rf benchmark/*_easyjson.go

build:
	go build -i -o ./bin/easyjson ./easyjson

generate: build
	bin/easyjson -stubs \
		./tests/snake.go \
		./tests/data.go \
		./tests/omitempty.go \
		./tests/nothing.go \
		./tests/named_type.go \
		./tests/custom_map_key_type.go \
		./tests/embedded_type.go \
		./tests/reference_to_pointer.go \
		./tests/html.go \
		./tests/unknown_fields.go \

	bin/easyjson -all ./tests/data.go
	bin/easyjson -all ./tests/nothing.go
	bin/easyjson -all ./tests/errors.go
	bin/easyjson -all ./tests/html.go
	bin/easyjson -snake_case ./tests/snake.go
	bin/easyjson -omit_empty ./tests/omitempty.go
	bin/easyjson -build_tags=use_easyjson ./benchmark/data.go
	bin/easyjson ./tests/nested_easy.go
	bin/easyjson ./tests/named_type.go
	bin/easyjson ./tests/custom_map_key_type.go
	bin/easyjson ./tests/embedded_type.go
	bin/easyjson ./tests/reference_to_pointer.go
	bin/easyjson ./tests/key_marshaler_map.go
	bin/easyjson -disallow_unknown_fields ./tests/disallow_unknown.go
	bin/easyjson ./tests/unknown_fields.go

test: generate
	go test \
		./tests \
		./jlexer \
		./gen \
		./buffer
	cd benchmark && go test -benchmem -tags use_easyjson -bench .
	golint -set_exit_status ./tests/*_easyjson.go

bench-other: generate
	cd benchmark && make

bench-python:
	benchmark/ujson.sh


.PHONY: clean generate test build
