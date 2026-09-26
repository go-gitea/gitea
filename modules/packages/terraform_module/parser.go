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
	"sync"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

var (
	ErrInvalidName         = errors.New("module name is invalid")
	ErrInvalidProvider     = errors.New("module provider is invalid")
	ErrInvalidVersion      = errors.New("module version is invalid")
	ErrArchiveTooLarge     = errors.New("module archive exceeds size limit")
	ErrNoRootModule        = errors.New("no terraform module at the archive root: package the module directory's contents, not the directory itself (e.g. `tar -czf module.tar.gz -C path/to/module .`)")
	ErrUnsupportedTFFormat = errors.New("only .tf files are supported, .tf.json is not")
)

const maxParseSize = 32 << 20 // decompressed bytes, bounds gzip bombs independently of the storage quota

// same rules as Terraform's module source address parser (github.com/hashicorp/terraform-registry-address)
var (
	nameRe   = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`\A[0-9A-Za-z](?:[0-9A-Za-z_-]{0,62}[0-9A-Za-z])?\z`) })
	systemRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`\A[0-9a-z]{1,64}\z`) })
)

func ValidateNameAndProvider(name, provider string) error {
	if !nameRe().MatchString(name) {
		return ErrInvalidName
	}
	if !systemRe().MatchString(provider) {
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
	sources := map[string]map[string][]byte{} // module dir ("" for the root) -> .tf filename -> content
	var readme string
	var hasTFJSON bool
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid tar stream: %w", err)
		}
		dir, base := path.Split(path.Clean(hdr.Name))
		dir = strings.TrimSuffix(dir, "/")
		if hdr.Typeflag != tar.TypeReg || strings.HasPrefix(base, "._") || !isModuleDir(dir) { // "._" are macOS AppleDouble sidecars
			continue
		}
		lowerBase := strings.ToLower(base)
		switch {
		case strings.HasSuffix(lowerBase, ".tf.json"):
			hasTFJSON = true
		case strings.HasSuffix(lowerBase, ".tf"):
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			if sources[dir] == nil {
				sources[dir] = map[string][]byte{}
			}
			sources[dir][base] = data
		case dir == "" && (lowerBase == "readme.md" || lowerBase == "readme"):
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			readme = string(data)
		}
	}

	if len(sources) == 0 {
		if hasTFJSON {
			return nil, ErrUnsupportedTFFormat
		}
		return nil, ErrNoRootModule
	}

	metadata := &Metadata{Readme: readme}
	for _, dir := range slices.Sorted(maps.Keys(sources)) {
		module, err := parseModule(sources[dir])
		if err != nil {
			return nil, err
		}
		if dir == "" {
			metadata.Root = module
		} else {
			module.Name = strings.TrimPrefix(dir, "modules/")
			metadata.Submodules = append(metadata.Submodules, module)
		}
	}
	return metadata, nil
}

// isModuleDir reports whether dir is the root or a `modules/<name>` submodule of the standard module structure
func isModuleDir(dir string) bool {
	name, ok := strings.CutPrefix(dir, "modules/")
	return dir == "" || ok && !strings.Contains(name, "/")
}

func parseModule(files map[string][]byte) (*Module, error) {
	module := &Module{}
	providers := map[string]*Provider{}
	parser := hclparse.NewParser()
	for _, filename := range slices.Sorted(maps.Keys(files)) {
		src := files[filename]
		file, diags := parser.ParseHCL(src, filename)
		if diags.HasErrors() {
			return nil, fmt.Errorf("parse %s: %w", filename, diags)
		}
		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, block := range body.Blocks {
			attrs := block.Body.Attributes
			switch {
			case block.Type == "variable" && len(block.Labels) == 1:
				module.Inputs = append(module.Inputs, &Input{
					Name:        block.Labels[0],
					Type:        attrSource(attrs["type"], src),
					Description: attrString(attrs["description"]),
					Default:     attrSource(attrs["default"], src),
					Required:    attrs["default"] == nil,
				})
			case block.Type == "output" && len(block.Labels) == 1:
				module.Outputs = append(module.Outputs, &Output{Name: block.Labels[0], Description: attrString(attrs["description"])})
			case block.Type == "terraform":
				for _, inner := range block.Body.Blocks {
					if inner.Type != "required_providers" {
						continue
					}
					for name, attr := range inner.Body.Attributes {
						if providers[name] == nil {
							providers[name] = parseProvider(name, attr.Expr)
						}
					}
				}
			}
		}
	}
	module.Providers = slices.SortedFunc(maps.Values(providers), func(a, b *Provider) int { return strings.Compare(a.Name, b.Name) })
	return module, nil
}

// parseProvider evaluates only the literal keys, `configuration_aliases` holds references that can't be evaluated
func parseProvider(name string, expr hcl.Expression) *Provider {
	provider := &Provider{Name: name}
	if version := exprString(expr); version != "" { // legacy `name = "<version>"` form
		provider.Version = version
		return provider
	}
	pairs, _ := hcl.ExprMap(expr)
	for _, pair := range pairs {
		switch exprString(pair.Key) {
		case "source":
			provider.Source = exprString(pair.Value)
		case "version":
			provider.Version = exprString(pair.Value)
		}
	}
	return provider
}

func exprString(expr hcl.Expression) string {
	val, diags := expr.Value(nil)
	if diags.HasErrors() || val.IsNull() || val.Type() != cty.String {
		return ""
	}
	return val.AsString()
}

func attrString(attr *hclsyntax.Attribute) string {
	if attr == nil {
		return ""
	}
	return exprString(attr.Expr)
}

// attrSource returns the source text of an expression like `list(string)` that can't be evaluated without a context
func attrSource(attr *hclsyntax.Attribute, src []byte) string {
	if attr == nil {
		return ""
	}
	return strings.TrimSpace(string(attr.Expr.Range().SliceBytes(src)))
}
