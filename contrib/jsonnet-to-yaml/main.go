package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-jsonnet"
	"gopkg.in/yaml.v3"
)

func main() {
	fname := ".drone.jsonnet"
	if len(os.Args) > 1 {
		fname = os.Args[1]
	}

	source, err := os.ReadFile(fname)
	if err != nil {
		panic(err)
	}

	vm := jsonnet.MakeVM()

	streams, err := vm.EvaluateSnippetStream(fname, string(source))
	if err != nil {
		panic(err)
	}

	var out strings.Builder
	for _, stream := range streams {
		var j any
		if err := yaml.Unmarshal([]byte(stream), &j); err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(j); err != nil {
			panic(err)
		}

		out.WriteString("---\n")
		out.Write(buf.Bytes())
	}
	fmt.Println(out.String())
}
