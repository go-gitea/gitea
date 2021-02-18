package code

import (
	"bytes"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
)

var mode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedImports |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo

// Packages is a wrapper around x/tools/go/packages that maintains a (hopefully prewarmed) cache of packages
// that can be invalidated as writes are made and packages are known to change.
type Packages struct {
	packages     map[string]*packages.Package
	importToName map[string]string
	loadErrors   []error

	numLoadCalls int // stupid test steam. ignore.
	numNameCalls int // stupid test steam. ignore.
}

// LoadAll will call packages.Load and return the package data for the given packages,
// but if the package already have been loaded it will return cached values instead.
func (p *Packages) LoadAll(importPaths ...string) []*packages.Package {
	if p.packages == nil {
		p.packages = map[string]*packages.Package{}
	}

	missing := make([]string, 0, len(importPaths))
	for _, path := range importPaths {
		if _, ok := p.packages[path]; ok {
			continue
		}
		missing = append(missing, path)
	}

	if len(missing) > 0 {
		p.numLoadCalls++
		pkgs, err := packages.Load(&packages.Config{Mode: mode}, missing...)
		if err != nil {
			p.loadErrors = append(p.loadErrors, err)
		}

		for _, pkg := range pkgs {
			p.addToCache(pkg)
		}
	}

	res := make([]*packages.Package, 0, len(importPaths))
	for _, path := range importPaths {
		res = append(res, p.packages[NormalizeVendor(path)])
	}
	return res
}

func (p *Packages) addToCache(pkg *packages.Package) {
	imp := NormalizeVendor(pkg.PkgPath)
	p.packages[imp] = pkg
	for _, imp := range pkg.Imports {
		if _, found := p.packages[NormalizeVendor(imp.PkgPath)]; !found {
			p.addToCache(imp)
		}
	}
}

// Load works the same as LoadAll, except a single package at a time.
func (p *Packages) Load(importPath string) *packages.Package {
	pkgs := p.LoadAll(importPath)
	if len(pkgs) == 0 {
		return nil
	}
	return pkgs[0]
}

// LoadWithTypes tries a standard load, which may not have enough type info (TypesInfo== nil) available if the imported package is a
// second order dependency. Fortunately this doesnt happen very often, so we can just issue a load when we detect it.
func (p *Packages) LoadWithTypes(importPath string) *packages.Package {
	pkg := p.Load(importPath)
	if pkg == nil || pkg.TypesInfo == nil {
		p.numLoadCalls++
		pkgs, err := packages.Load(&packages.Config{Mode: mode}, importPath)
		if err != nil {
			p.loadErrors = append(p.loadErrors, err)
			return nil
		}
		p.addToCache(pkgs[0])
		pkg = pkgs[0]
	}
	return pkg
}

// NameForPackage looks up the package name from the package stanza in the go files at the given import path.
func (p *Packages) NameForPackage(importPath string) string {
	if importPath == "" {
		panic(errors.New("import path can not be empty"))
	}
	if p.importToName == nil {
		p.importToName = map[string]string{}
	}

	importPath = NormalizeVendor(importPath)

	// if its in the name cache use it
	if name := p.importToName[importPath]; name != "" {
		return name
	}

	// otherwise we might have already loaded the full package data for it cached
	pkg := p.packages[importPath]

	if pkg == nil {
		// otherwise do a name only lookup for it but dont put it in the package cache.
		p.numNameCalls++
		pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName}, importPath)
		if err != nil {
			p.loadErrors = append(p.loadErrors, err)
		} else {
			pkg = pkgs[0]
		}
	}

	if pkg == nil || pkg.Name == "" {
		return SanitizePackageName(filepath.Base(importPath))
	}

	p.importToName[importPath] = pkg.Name

	return pkg.Name
}

// Evict removes a given package import path from the cache, along with any packages that depend on it. Further calls
// to Load will fetch it from disk.
func (p *Packages) Evict(importPath string) {
	delete(p.packages, importPath)

	for _, pkg := range p.packages {
		for _, imported := range pkg.Imports {
			if imported.PkgPath == importPath {
				p.Evict(pkg.PkgPath)
			}
		}
	}
}

// Errors returns any errors that were returned by Load, either from the call itself or any of the loaded packages.
func (p *Packages) Errors() PkgErrors {
	var res []error //nolint:prealloc
	res = append(res, p.loadErrors...)
	for _, pkg := range p.packages {
		for _, err := range pkg.Errors {
			res = append(res, err)
		}
	}
	return res
}

type PkgErrors []error

func (p PkgErrors) Error() string {
	var b bytes.Buffer
	b.WriteString("packages.Load: ")
	for _, e := range p {
		b.WriteString(e.Error() + "\n")
	}
	return b.String()
}
