package templates

import (
	"fmt"
	"go/types"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/internal/code"
)

type Import struct {
	Name  string
	Path  string
	Alias string
}

type Imports struct {
	imports  []*Import
	destDir  string
	packages *code.Packages
}

func (i *Import) String() string {
	if strings.HasSuffix(i.Path, i.Alias) {
		return strconv.Quote(i.Path)
	}

	return i.Alias + " " + strconv.Quote(i.Path)
}

func (s *Imports) String() string {
	res := ""
	for i, imp := range s.imports {
		if i != 0 {
			res += "\n"
		}
		res += imp.String()
	}
	return res
}

func (s *Imports) Reserve(path string, aliases ...string) (string, error) {
	if path == "" {
		panic("empty ambient import")
	}

	// if we are referencing our own package we dont need an import
	if code.ImportPathForDir(s.destDir) == path {
		return "", nil
	}

	name := s.packages.NameForPackage(path)
	var alias string
	if len(aliases) != 1 {
		alias = name
	} else {
		alias = aliases[0]
	}

	if existing := s.findByPath(path); existing != nil {
		if existing.Alias == alias {
			return "", nil
		}
		return "", fmt.Errorf("ambient import already exists")
	}

	if alias := s.findByAlias(alias); alias != nil {
		return "", fmt.Errorf("ambient import collides on an alias")
	}

	s.imports = append(s.imports, &Import{
		Name:  name,
		Path:  path,
		Alias: alias,
	})

	return "", nil
}

func (s *Imports) Lookup(path string) string {
	if path == "" {
		return ""
	}

	path = code.NormalizeVendor(path)

	// if we are referencing our own package we dont need an import
	if code.ImportPathForDir(s.destDir) == path {
		return ""
	}

	if existing := s.findByPath(path); existing != nil {
		return existing.Alias
	}

	imp := &Import{
		Name: s.packages.NameForPackage(path),
		Path: path,
	}
	s.imports = append(s.imports, imp)

	alias := imp.Name
	i := 1
	for s.findByAlias(alias) != nil {
		alias = imp.Name + strconv.Itoa(i)
		i++
		if i > 10 {
			panic(fmt.Errorf("too many collisions, last attempt was %s", alias))
		}
	}
	imp.Alias = alias

	return imp.Alias
}

func (s *Imports) LookupType(t types.Type) string {
	return types.TypeString(t, func(i *types.Package) string {
		return s.Lookup(i.Path())
	})
}

func (s Imports) findByPath(importPath string) *Import {
	for _, imp := range s.imports {
		if imp.Path == importPath {
			return imp
		}
	}
	return nil
}

func (s Imports) findByAlias(alias string) *Import {
	for _, imp := range s.imports {
		if imp.Alias == alias {
			return imp
		}
	}
	return nil
}
