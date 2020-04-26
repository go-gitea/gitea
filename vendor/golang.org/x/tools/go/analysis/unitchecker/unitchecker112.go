// +build go1.12

package unitchecker
import "code.gitea.io/gitea/traceinit"

import "go/importer"

func init() {
traceinit.Trace("./vendor/golang.org/x/tools/go/analysis/unitchecker/unitchecker112.go")
	importerForCompiler = importer.ForCompiler
}
