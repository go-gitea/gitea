// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"gitea.dev/modules/container"

	"github.com/hashicorp/go-version"
)

var (
	ErrInvalidName     = errors.New("module name is invalid")
	ErrInvalidProvider = errors.New("module provider is invalid")
	ErrInvalidVersion  = errors.New("module version is invalid")
	ErrArchiveTooLarge = errors.New("module archive exceeds size limit")
	ErrNoRootModule    = errors.New("no terraform module at the archive root: package the module directory's contents, not the directory itself (e.g. `tar -czf module.tar.gz -C path/to/module .`)")
)

const maxParseSize = 32 << 20 // decompressed bytes, bounds gzip bombs independently of the storage quota

// same rules as Terraform's module source address parser (github.com/hashicorp/terraform-registry-address)
var (
	nameRe   = regexp.MustCompile(`\A[0-9A-Za-z](?:[0-9A-Za-z_-]{0,62}[0-9A-Za-z])?\z`)
	systemRe = regexp.MustCompile(`\A[0-9a-z]{1,64}\z`)
)

func ValidateNameAndProvider(name, provider string) error {
	if !nameRe.MatchString(name) {
		return ErrInvalidName
	}
	if !systemRe.MatchString(provider) {
		return ErrInvalidProvider
	}
	return nil
}

// NormalizeVersion returns the canonical semver form, so `v1.0.0` and `1.0.0` are the same version
func NormalizeVersion(s string) (string, error) {
	v, err := version.NewSemver(s)
	if err != nil {
		return "", ErrInvalidVersion
	}
	return v.String(), nil
}

func ParseModuleArchive(r io.Reader) (*Metadata, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("invalid gzip stream: %w", err)
	}
	defer gz.Close()

	lr := &io.LimitedReader{R: gz, N: maxParseSize}
	metadata, err := parseTar(tar.NewReader(lr))
	if lr.N <= 0 {
		return nil, ErrArchiveTooLarge
	}
	return metadata, err
}

func parseTar(tr *tar.Reader) (*Metadata, error) {
	metadata := &Metadata{}
	root := map[string][]byte{} // .tf.json content yields no HCL blocks
	submodules := container.Set[string]{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid tar stream: %w", err)
		}
		dir, base := path.Split(path.Clean(hdr.Name))
		if hdr.Typeflag != tar.TypeReg || strings.HasPrefix(base, "._") { // macOS AppleDouble sidecars
			continue
		}
		dir = strings.TrimSuffix(dir, "/")
		lowerBase := strings.ToLower(base)
		isSource := strings.HasSuffix(lowerBase, ".tf") || strings.HasSuffix(lowerBase, ".tf.json")
		isReadme := lowerBase == "readme.md" || lowerBase == "readme"
		if name, ok := strings.CutPrefix(dir, "modules/"); ok && isSource && !strings.Contains(name, "/") { // standard module structure
			submodules.Add(name)
		}
		if dir != "" || !isSource && !isReadme {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		if isReadme {
			metadata.Readme = string(data)
		} else {
			root[base] = data
		}
	}
	if len(root) == 0 && len(submodules) == 0 {
		return nil, ErrNoRootModule
	}
	if len(root) > 0 {
		metadata.Root = parseModule(root)
	}
	metadata.Submodules = slices.Sorted(maps.Keys(submodules))
	return metadata, nil
}

func parseModule(files map[string][]byte) *Module {
	module := &Module{}
	providers := map[string]*Provider{}
	for _, filename := range slices.Sorted(maps.Keys(files)) {
		for _, block := range (&hclScanner{src: files[filename]}).body(0).blocks {
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
