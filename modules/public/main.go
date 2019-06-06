// +build ignore

package main

import (
	"log"
	"net/http"

	"github.com/shurcooL/vfsgen"
)

func main() {
	var fsPublic http.FileSystem = http.Dir("../../public")
	err := vfsgen.Generate(fsPublic, vfsgen.Options{
		PackageName:  "public",
		BuildTags:    "bindata",
		VariableName: "Assets",
		Filename:     "bindata.go",
	})
	if err != nil {
		log.Fatal("%v", err)
	}
}
