package code

import (
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

var gopaths []string

func init() {
	gopaths = filepath.SplitList(build.Default.GOPATH)
	for i, p := range gopaths {
		gopaths[i] = filepath.ToSlash(filepath.Join(p, "src"))
	}
}

// NameForDir manually looks for package stanzas in files located in the given directory. This can be
// much faster than having to consult go list, because we already know exactly where to look.
func NameForDir(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return SanitizePackageName(filepath.Base(dir))
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return SanitizePackageName(filepath.Base(dir))
	}
	fset := token.NewFileSet()
	for _, file := range files {
		if !strings.HasSuffix(strings.ToLower(file.Name()), ".go") {
			continue
		}

		filename := filepath.Join(dir, file.Name())
		if src, err := parser.ParseFile(fset, filename, nil, parser.PackageClauseOnly); err == nil {
			return src.Name.Name
		}
	}

	return SanitizePackageName(filepath.Base(dir))
}

// goModuleRoot returns the root of the current go module if there is a go.mod file in the directory tree
// If not, it returns false
func goModuleRoot(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}
	dir = filepath.ToSlash(dir)
	modDir := dir
	assumedPart := ""
	for {
		f, err := ioutil.ReadFile(filepath.Join(modDir, "go.mod"))
		if err == nil {
			// found it, stop searching
			return string(modregex.FindSubmatch(f)[1]) + assumedPart, true
		}

		assumedPart = "/" + filepath.Base(modDir) + assumedPart
		parentDir, err := filepath.Abs(filepath.Join(modDir, ".."))
		if err != nil {
			panic(err)
		}

		if parentDir == modDir {
			// Walked all the way to the root and didnt find anything :'(
			break
		}
		modDir = parentDir
	}
	return "", false
}

// ImportPathForDir takes a path and returns a golang import path for the package
func ImportPathForDir(dir string) (res string) {
	dir, err := filepath.Abs(dir)

	if err != nil {
		panic(err)
	}
	dir = filepath.ToSlash(dir)

	modDir, ok := goModuleRoot(dir)
	if ok {
		return modDir
	}

	for _, gopath := range gopaths {
		if len(gopath) < len(dir) && strings.EqualFold(gopath, dir[0:len(gopath)]) {
			return dir[len(gopath)+1:]
		}
	}

	return ""
}

var modregex = regexp.MustCompile(`module ([^\s]*)`)
