package main

import (
	"net/http"
	"os"

	"code.gitea.io/gitea/modules/log"
	"github.com/shurcooL/vfsgen"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "public" {
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

	if len(os.Args) == 2 && os.Args[1] == "templates" {
		var fsTemplates http.FileSystem = http.Dir("../../templates")
		err := vfsgen.Generate(fsTemplates, vfsgen.Options{
			PackageName:  "templates",
			BuildTags:    "bindata",
			VariableName: "Assets",
			Filename:     "bindata.go",
		})
		if err != nil {
			log.Fatal("%v", err)
		}
	}

	if len(os.Args) == 2 && os.Args[1] == "options" {
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
}
