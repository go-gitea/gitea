// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"

	packages_module "gitea.dev/modules/packages"

	"github.com/hashicorp/go-version"
)

var (
	ErrInvalidNamespace = errors.New("module namespace (owner name) is invalid")
	ErrInvalidName      = errors.New("module name is invalid")
	ErrInvalidProvider  = errors.New("module provider is invalid")
	ErrInvalidVersion   = errors.New("module version is invalid")
	ErrNoRootModule     = errors.New("no terraform module at the archive root: package the module directory's contents, not the directory itself (e.g. `tar -czf module.tar.gz -C path/to/module .`)")
)

const maxReadmeSize = 1024 * 1024 // larger READMEs are not stored, they would be rendered on every page view

var globalVars = sync.OnceValue(func() (ret struct {
	nameRe   *regexp.Regexp
	systemRe *regexp.Regexp
},
) {
	// same rules as Terraform's module source address parser (github.com/hashicorp/terraform-registry-address)
	ret.nameRe = regexp.MustCompile(`\A[0-9A-Za-z](?:[0-9A-Za-z_-]{0,62}[0-9A-Za-z])?\z`)
	ret.systemRe = regexp.MustCompile(`\A[0-9a-z]{1,64}\z`)
	return ret
})

func ValidateAddress(namespace, name, provider string) error {
	if !globalVars().nameRe.MatchString(namespace) {
		return ErrInvalidNamespace
	}
	if !globalVars().nameRe.MatchString(name) {
		return ErrInvalidName
	}
	if !globalVars().systemRe.MatchString(provider) {
		return ErrInvalidProvider
	}
	return nil
}

// NormalizeVersion returns the canonical semver form, so `v1.0.0` and `1.0.0` are the same version
func NormalizeVersion(raw string) (string, error) {
	parsed, err := version.NewSemver(raw)
	if err != nil {
		return "", ErrInvalidVersion
	}
	return parsed.String(), nil
}

func ParseModuleArchive(r io.Reader) (*Metadata, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("invalid gzip stream: %w", err)
	}
	defer gz.Close()
	return parseTar(tar.NewReader(packages_module.NewLimitedReader(gz, packages_module.MaxMetadataSize)))
}

func parseTar(tr *tar.Reader) (*Metadata, error) {
	retained := packages_module.NewLimitedReader(tr, packages_module.MaxMetadataSize) // sparse entries read more than they decompress
	metadata := &Metadata{}
	root := map[string][]byte{} // .tf.json content yields no HCL blocks
	submodules := map[string]map[string][]byte{}
	subReadmes := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid tar stream: %w", err)
		}
		dir, base := path.Split(path.Clean(hdr.Name))
		if hdr.Typeflag != tar.TypeReg || strings.HasPrefix(base, ".") { // Terraform ignores dotfiles
			continue
		}
		dir = strings.TrimSuffix(dir, "/")
		isSource := strings.HasSuffix(base, ".tf") || strings.HasSuffix(base, ".tf.json")
		isReadme := strings.EqualFold(base, "readme.md") || strings.EqualFold(base, "readme")
		name, isSubmodule := strings.CutPrefix(dir, "modules/")
		isSubmodule = isSubmodule && !strings.Contains(name, "/") // standard module structure, one level deep
		if !isSource && !isReadme || dir != "" && !isSubmodule || isReadme && hdr.Size > maxReadmeSize {
			continue
		}
		data, err := io.ReadAll(retained)
		if err != nil {
			return nil, err
		}
		switch {
		case isSubmodule && isSource:
			if submodules[name] == nil {
				submodules[name] = map[string][]byte{}
			}
			submodules[name][base] = data
		case isSubmodule:
			subReadmes[name] = string(data)
		case isReadme:
			metadata.Readme = string(data)
		default:
			root[base] = data
		}
	}
	if len(root) == 0 && len(submodules) == 0 {
		return nil, ErrNoRootModule
	}
	if len(root) > 0 {
		metadata.Root = parseModule(root)
	}
	if len(submodules) > 0 {
		metadata.Submodules = make(map[string]*Submodule, len(submodules))
		for name, files := range submodules {
			metadata.Submodules[name] = &Submodule{Readme: subReadmes[name], Module: parseModule(files)}
		}
	}
	return metadata, nil
}

func parseModule(files map[string][]byte) *Module {
	module := &Module{}
	providers := map[string]*Provider{}
	for _, filename := range slices.Sorted(maps.Keys(files)) {
		for _, block := range (&hclScanner{src: bytes.TrimPrefix(files[filename], []byte("\xef\xbb\xbf"))}).body(0).blocks {
			attrs := block.attrs
			switch {
			case block.typ == "variable" && len(block.labels) == 1:
				_, hasDefault := attrs["default"]
				module.Inputs = append(module.Inputs, &Input{
					Name:        block.labels[0],
					Type:        attrs["type"],
					Description: hclString(attrs["description"]),
					Default:     attrs["default"],
					Required:    !hasDefault,
				})
			case block.typ == "output" && len(block.labels) == 1:
				module.Outputs = append(module.Outputs, &Output{Name: block.labels[0], Description: hclString(attrs["description"])})
			case block.typ == "terraform":
				for _, inner := range block.blocks {
					if inner.typ != "required_providers" {
						continue
					}
					for name, expr := range inner.attrs {
						if providers[name] != nil {
							continue
						}
						provider := &Provider{Name: name}
						if strings.HasPrefix(expr, "{") {
							fields := (&hclScanner{src: []byte(expr[1:])}).body(2).attrs
							provider.Source, provider.Version = hclString(fields["source"]), hclString(fields["version"])
						} else {
							provider.Version = hclString(expr) // legacy `name = "<version>"` form
						}
						providers[name] = provider
					}
				}
			}
		}
	}
	module.Providers = slices.SortedFunc(maps.Values(providers), func(a, b *Provider) int { return strings.Compare(a.Name, b.Name) })
	return module
}
