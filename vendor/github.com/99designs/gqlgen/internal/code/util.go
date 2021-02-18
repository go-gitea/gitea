package code

import (
	"go/build"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// take a string in the form github.com/package/blah.Type and split it into package and type
func PkgAndType(name string) (string, string) {
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		return "", name
	}

	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

var modsRegex = regexp.MustCompile(`^(\*|\[\])*`)

// NormalizeVendor takes a qualified package path and turns it into normal one.
// eg .
// github.com/foo/vendor/github.com/99designs/gqlgen/graphql becomes
// github.com/99designs/gqlgen/graphql
func NormalizeVendor(pkg string) string {
	modifiers := modsRegex.FindAllString(pkg, 1)[0]
	pkg = strings.TrimPrefix(pkg, modifiers)
	parts := strings.Split(pkg, "/vendor/")
	return modifiers + parts[len(parts)-1]
}

// QualifyPackagePath takes an import and fully qualifies it with a vendor dir, if one is required.
// eg .
// github.com/99designs/gqlgen/graphql becomes
// github.com/foo/vendor/github.com/99designs/gqlgen/graphql
//
// x/tools/packages only supports 'qualified package paths' so this will need to be done prior to calling it
// See https://github.com/golang/go/issues/30289
func QualifyPackagePath(importPath string) string {
	wd, _ := os.Getwd()

	// in go module mode, the import path doesn't need fixing
	if _, ok := goModuleRoot(wd); ok {
		return importPath
	}

	pkg, err := build.Import(importPath, wd, 0)
	if err != nil {
		return importPath
	}

	return pkg.ImportPath
}

var invalidPackageNameChar = regexp.MustCompile(`[^\w]`)

func SanitizePackageName(pkg string) string {
	return invalidPackageNameChar.ReplaceAllLiteralString(filepath.Base(pkg), "_")
}
