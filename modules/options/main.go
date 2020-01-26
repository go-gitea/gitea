// +build ignore

package main

import (
	"log"
	"net/http"

	"github.com/shurcooL/vfsgen"
)

func main() {
	var fsTemplates http.FileSystem = http.Dir("../../options")
	err := vfsgen.Generate(fsTemplates, vfsgen.Options{
		PackageName:  "options",
		BuildTags:    "bindata",
		VariableName: "Assets",
		Filename:     "bindata.go",
	})
	if err != nil {
		log.Fatal("%v", err)
	}
}
