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
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
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
	root := map[string][]byte{}
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
		var err error
		if metadata.Root, err = parseModule(root); err != nil {
			return nil, err
		}
	}
	metadata.Submodules = slices.Sorted(maps.Keys(submodules))
	return metadata, nil
}

func parseModule(files map[string][]byte) (*Module, error) {
	module := &Module{}
	providers := map[string]*Provider{}
	parser := hclparse.NewParser()
	for _, filename := range slices.Sorted(maps.Keys(files)) {
		if strings.HasSuffix(strings.ToLower(filename), ".json") {
			continue // .tf.json is consumable but not indexed
		}
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
