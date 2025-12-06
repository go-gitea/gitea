package main

import (
	"fmt"
	"strings"
	"syscall/js"
)

func processFile(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return js.ValueOf("(no content)")
	}
	content := args[0].String()
	lines := strings.Split(content, "\n")
	var b strings.Builder
	b.Grow(len(content) + len(lines)*8)
	for i, line := range lines {
		fmt.Fprintf(&b, "%4d │ %s\n", i+1, strings.ToUpper(line))
	}
	return js.ValueOf(b.String())
}

func main() {
	js.Global().Set("wasmProcessFile", js.FuncOf(processFile))
	select {}
}
