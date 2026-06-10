// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Validation errors returned by the parser.
var (
	ErrInvalidName         = errors.New("module name is invalid")
	ErrInvalidProvider     = errors.New("module provider is invalid")
	ErrInvalidVersion      = errors.New("module version is invalid")
	ErrArchiveTooLarge     = errors.New("module archive exceeds size limit")
	ErrUnsafeArchivePath   = errors.New("module archive contains an unsafe file path")
	ErrEmptyModule         = errors.New("module archive contains no .tf files at its root or in a single top-level directory; package the module so its .tf files are at the archive root (e.g. `tar -czf module.tgz *`) or wrapped in one directory")
	ErrAmbiguousModuleRoot = errors.New("module archive has multiple top-level directories and no .tf files at its root; cannot determine the module root")
	ErrUnsupportedTFFormat = errors.New("only .tf files are supported (.tf.json is not parsed in v1)")
)

// maxParseSize is a hard ceiling on the total decompressed bytes read
// while parsing an archive, independent of the configurable storage
// quota (LIMIT_SIZE_TERRAFORM_MODULE). It guards against gzip bombs even
// when the operator disables the storage quota with -1. Real Terraform
// modules are KB-scale, so 32 MiB is generous.
const maxParseSize = 32 << 20 // 32 MiB

// Module is the result of parsing a Terraform module archive.
type Module struct {
	Metadata *Metadata
}

// HashiCorp constrains module name and provider to lowercase alphanumeric
// plus underscores/dashes. The namespace is a Gitea user/org and is
// validated by the user lookup instead. See:
// https://developer.hashicorp.com/terraform/internals/module-registry-protocol
var (
	nameRe = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`\A[a-z0-9][a-z0-9_-]{0,63}\z`)
	})
	providerRe = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`\A[a-z0-9][a-z0-9-]{0,63}\z`)
	})
)

// ValidateName returns ErrInvalidName for non-conforming module names.
func ValidateName(s string) error {
	if !nameRe().MatchString(s) {
		return ErrInvalidName
	}
	return nil
}

// ValidateProvider returns ErrInvalidProvider for non-conforming provider segments.
func ValidateProvider(s string) error {
	if !providerRe().MatchString(s) {
		return ErrInvalidProvider
	}
	return nil
}

// dirFiles holds the parse-relevant files collected for a single
// directory level of the archive (the root, or a top-level directory).
type dirFiles struct {
	tf     map[string][]byte // basename -> .tf source
	readme string
	tfJSON bool // a .tf.json was present (unsupported in v1)
}

// ParseModuleArchive consumes a gzipped tar archive and extracts the root
// module's metadata. The module sources may sit either at the archive
// root (`tar -czf module.tgz *`) or wrapped in a single top-level
// directory (a GitHub release tarball, `git archive --prefix`, ...). The
// detected layout is reported via Metadata.ModuleDir so the download
// handler can serve the archive with the matching go-getter subdir.
//
// maxSize caps the total uncompressed bytes read; values <= 0 (e.g. an
// unlimited storage quota) or above maxParseSize are clamped to
// maxParseSize so a gzip bomb can never be fully buffered into memory.
func ParseModuleArchive(r io.Reader, maxSize int64) (*Module, error) {
	if maxSize <= 0 || maxSize > maxParseSize {
		maxSize = maxParseSize
	}

	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("invalid gzip stream: %w", err)
	}
	defer gz.Close()

	var (
		tr       = tar.NewReader(gz)
		consumed int64
		// byDir maps a directory level ("" for the archive root, or a
		// top-level directory name) to its collected files.
		byDir = map[string]*dirFiles{}
		// topDirs is the set of distinct top-level directory names, used
		// to detect the single-wrapper-directory layout.
		topDirs = map[string]struct{}{}
	)

	dirEntry := func(dir string) *dirFiles {
		df := byDir[dir]
		if df == nil {
			df = &dirFiles{tf: map[string][]byte{}}
			byDir[dir] = df
		}
		return df
	}

	// handleFile reads .tf and README payloads for the given directory
	// level and discards everything else, while always counting bytes
	// against the size cap.
	handleFile := func(dir, base string) error {
		lower := strings.ToLower(base)
		switch {
		case strings.HasSuffix(lower, ".tf.json"):
			n, err := skipCapped(tr, maxSize, consumed)
			if err != nil {
				return err
			}
			consumed += n
			dirEntry(dir).tfJSON = true
		case strings.HasSuffix(lower, ".tf"):
			data, n, err := readCapped(tr, maxSize, consumed)
			if err != nil {
				return err
			}
			consumed += n
			dirEntry(dir).tf[base] = data
		case lower == "readme.md" || lower == "readme":
			data, n, err := readCapped(tr, maxSize, consumed)
			if err != nil {
				return err
			}
			consumed += n
			dirEntry(dir).readme = string(data)
		default:
			n, err := skipCapped(tr, maxSize, consumed)
			if err != nil {
				return err
			}
			consumed += n
		}
		return nil
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}

		// Reject absolute and traversing paths before we touch the file.
		clean := path.Clean(hdr.Name)
		if path.IsAbs(clean) || strings.HasPrefix(clean, "../") || clean == ".." {
			return nil, ErrUnsafeArchivePath
		}
		if clean == "." || isArchiveJunk(clean) {
			if hdr.Typeflag == tar.TypeReg {
				n, err := skipCapped(tr, maxSize, consumed)
				if err != nil {
					return nil, err
				}
				consumed += n
			}
			continue
		}

		// Record top-level directories so we can spot a single wrapper dir.
		if strings.Contains(clean, "/") {
			topDirs[clean[:strings.IndexByte(clean, '/')]] = struct{}{}
		} else if hdr.Typeflag == tar.TypeDir {
			topDirs[clean] = struct{}{}
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		// Only the archive root (depth 0) and one level deep (depth 1,
		// the wrapper directory) can hold the root module. Anything
		// deeper is a submodule or example and is skipped in v1.
		switch strings.Count(clean, "/") {
		case 0:
			if err := handleFile("", clean); err != nil {
				return nil, err
			}
		case 1:
			dir, base, _ := strings.Cut(clean, "/")
			if err := handleFile(dir, base); err != nil {
				return nil, err
			}
		default:
			n, err := skipCapped(tr, maxSize, consumed)
			if err != nil {
				return nil, err
			}
			consumed += n
		}
	}

	moduleDir, df, err := selectModuleRoot(byDir, topDirs)
	if err != nil {
		return nil, err
	}

	root, description, err := parseRoot(df.tf)
	if err != nil {
		return nil, err
	}

	return &Module{
		Metadata: &Metadata{
			Description: description,
			Readme:      df.readme,
			Root:        root,
			Providers:   root.Providers,
			ModuleDir:   moduleDir,
		},
	}, nil
}

// selectModuleRoot picks the directory level that holds the root module.
// Root-level .tf files win (flat layout); otherwise a single top-level
// directory containing .tf files is the module root (wrapped layout).
func selectModuleRoot(byDir map[string]*dirFiles, topDirs map[string]struct{}) (string, *dirFiles, error) {
	if df := byDir[""]; df != nil && len(df.tf) > 0 {
		return "", df, nil
	}
	if len(topDirs) == 1 {
		var only string
		for d := range topDirs {
			only = d
		}
		if df := byDir[only]; df != nil && len(df.tf) > 0 {
			return only, df, nil
		}
		if df := byDir[only]; df != nil && df.tfJSON {
			return "", nil, ErrUnsupportedTFFormat
		}
		return "", nil, ErrEmptyModule
	}
	// No usable .tf at the root or in a single wrapper directory.
	if df := byDir[""]; df != nil && df.tfJSON {
		return "", nil, ErrUnsupportedTFFormat
	}
	if len(topDirs) > 1 {
		return "", nil, ErrAmbiguousModuleRoot
	}
	return "", nil, ErrEmptyModule
}

// isArchiveJunk reports whether a cleaned path is packaging cruft that
// must never be treated as module source: macOS AppleDouble sidecars
// (`._foo`) and `__MACOSX/` entries, plus VCS/state directories that
// release tooling sometimes leaks (`.git/`, `.terraform/`).
func isArchiveJunk(clean string) bool {
	if strings.HasPrefix(path.Base(clean), "._") {
		return true
	}
	for comp := range strings.SplitSeq(clean, "/") {
		switch comp {
		case "__MACOSX", ".git", ".terraform":
			return true
		}
	}
	return false
}

// readCapped reads the current tar entry into memory and rejects once
// the running total would exceed maxSize. maxSize == 0 disables the cap.
func readCapped(tr *tar.Reader, maxSize, consumed int64) ([]byte, int64, error) {
	if maxSize <= 0 {
		data, err := io.ReadAll(tr)
		return data, int64(len(data)), err
	}
	remaining := maxSize - consumed
	if remaining <= 0 {
		return nil, 0, ErrArchiveTooLarge
	}
	// +1 byte so we can detect overflow instead of silently truncating.
	data, err := io.ReadAll(io.LimitReader(tr, remaining+1))
	if err != nil {
		return nil, 0, err
	}
	if int64(len(data)) > remaining {
		return nil, 0, ErrArchiveTooLarge
	}
	return data, int64(len(data)), nil
}

// skipCapped discards an entry's bytes while still counting them
// against the archive size limit.
func skipCapped(tr *tar.Reader, maxSize, consumed int64) (int64, error) {
	if maxSize <= 0 {
		return io.Copy(io.Discard, tr)
	}
	remaining := maxSize - consumed
	if remaining <= 0 {
		return 0, ErrArchiveTooLarge
	}
	n, err := io.Copy(io.Discard, io.LimitReader(tr, remaining+1))
	if err != nil {
		return 0, err
	}
	if n > remaining {
		return 0, ErrArchiveTooLarge
	}
	return n, nil
}

// parseRoot parses every .tf file in the root module and aggregates
// inputs, outputs, resources, data sources, sub-module references and
// provider requirements.
func parseRoot(files map[string][]byte) (*Root, string, error) {
	parser := hclparse.NewParser()
	root := &Root{}
	var (
		description    string
		providersAccum []*ProviderRequirement
	)

	// Deterministic order keeps tests stable.
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		src := files[name]
		file, diags := parser.ParseHCL(src, name)
		if diags.HasErrors() {
			return nil, "", fmt.Errorf("parse %s: %s", name, diags.Error())
		}
		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, block := range body.Blocks {
			switch block.Type {
			case "variable":
				if in := extractInput(block, src); in != nil {
					root.Inputs = append(root.Inputs, in)
				}
			case "output":
				if out := extractOutput(block); out != nil {
					root.Outputs = append(root.Outputs, out)
				}
			case "resource":
				if res := extractResource(block, false); res != nil {
					root.Resources = append(root.Resources, res)
				}
			case "data":
				if res := extractResource(block, true); res != nil {
					root.Resources = append(root.Resources, res)
				}
			case "module":
				if dep := extractModuleRef(block); dep != nil {
					root.Dependencies = append(root.Dependencies, dep)
				}
			case "terraform":
				providersAccum = extractTerraformBlock(block, root, &description, providersAccum)
			}
		}
	}

	root.Providers = dedupAndSortProviders(providersAccum)
	return root, description, nil
}

func extractInput(block *hclsyntax.Block, src []byte) *Input {
	if len(block.Labels) == 0 {
		return nil
	}
	in := &Input{Name: block.Labels[0], Required: true}
	if attr, ok := block.Body.Attributes["description"]; ok {
		in.Description = stringValue(attr)
	}
	if attr, ok := block.Body.Attributes["type"]; ok {
		in.Type = exprSource(attr, src)
	}
	if attr, ok := block.Body.Attributes["default"]; ok {
		in.Default = exprSource(attr, src)
		in.Required = false
	}
	if attr, ok := block.Body.Attributes["sensitive"]; ok {
		in.Sensitive = boolValue(attr)
	}
	return in
}

func extractOutput(block *hclsyntax.Block) *Output {
	if len(block.Labels) == 0 {
		return nil
	}
	out := &Output{Name: block.Labels[0]}
	if attr, ok := block.Body.Attributes["description"]; ok {
		out.Description = stringValue(attr)
	}
	if attr, ok := block.Body.Attributes["sensitive"]; ok {
		out.Sensitive = boolValue(attr)
	}
	return out
}

func extractResource(block *hclsyntax.Block, isData bool) *Resource {
	if len(block.Labels) < 2 {
		return nil
	}
	prefix := "resource"
	if isData {
		prefix = "data"
	}
	return &Resource{
		Type:    block.Labels[0],
		Name:    block.Labels[1],
		IsData:  isData,
		Address: fmt.Sprintf("%s.%s.%s", prefix, block.Labels[0], block.Labels[1]),
	}
}

func extractModuleRef(block *hclsyntax.Block) *ModuleReference {
	if len(block.Labels) == 0 {
		return nil
	}
	ref := &ModuleReference{Name: block.Labels[0]}
	if attr, ok := block.Body.Attributes["source"]; ok {
		ref.Source = stringValue(attr)
	}
	if attr, ok := block.Body.Attributes["version"]; ok {
		ref.Version = stringValue(attr)
	}
	return ref
}

// extractTerraformBlock pulls required_version, an optional description
// and required_providers entries out of a `terraform { }` block.
// Returns the (possibly appended-to) accumulator of providers.
func extractTerraformBlock(block *hclsyntax.Block, root *Root, description *string, acc []*ProviderRequirement) []*ProviderRequirement {
	if attr, ok := block.Body.Attributes["required_version"]; ok {
		if s := stringValue(attr); s != "" {
			root.RequiredCore = append(root.RequiredCore, s)
		}
	}
	if attr, ok := block.Body.Attributes["description"]; ok && *description == "" {
		*description = stringValue(attr)
	}
	for _, inner := range block.Body.Blocks {
		if inner.Type != "required_providers" {
			continue
		}
		for name, attr := range inner.Body.Attributes {
			req := &ProviderRequirement{Name: name}
			if val, diags := attr.Expr.Value(nil); !diags.HasErrors() && !val.IsNull() {
				switch {
				case val.Type() == cty.String:
					req.VersionConstraint = val.AsString()
				case val.Type().IsObjectType() || val.Type().IsMapType():
					if val.Type().HasAttribute("source") {
						req.Source = ctyString(val.GetAttr("source"))
					}
					if val.Type().HasAttribute("version") {
						req.VersionConstraint = ctyString(val.GetAttr("version"))
					}
				}
			}
			acc = append(acc, req)
		}
	}
	return acc
}

func dedupAndSortProviders(in []*ProviderRequirement) []*ProviderRequirement {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]*ProviderRequirement, 0, len(in))
	for _, p := range in {
		if _, ok := seen[p.Name]; ok {
			continue
		}
		seen[p.Name] = struct{}{}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// stringValue returns the literal string contents of attr or "" if the
// expression is not a simple string literal.
func stringValue(attr *hclsyntax.Attribute) string {
	val, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || val.IsNull() || val.Type() != cty.String {
		return ""
	}
	return val.AsString()
}

// boolValue returns the literal boolean contents of attr, false otherwise.
func boolValue(attr *hclsyntax.Attribute) bool {
	val, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || val.IsNull() || val.Type() != cty.Bool {
		return false
	}
	return val.True()
}

// exprSource returns the source text spanning the attribute's
// expression. We need this for `type` (e.g. `list(string)`) and
// `default` values, which HCL otherwise refuses to evaluate without
// a populated EvalContext.
func exprSource(attr *hclsyntax.Attribute, src []byte) string {
	rng := attr.Expr.Range()
	start, end := rng.Start.Byte, rng.End.Byte
	if start < 0 || end > len(src) || start >= end {
		return ""
	}
	return strings.TrimSpace(string(src[start:end]))
}

func ctyString(v cty.Value) string {
	if v.IsNull() || v.Type() != cty.String {
		return ""
	}
	return v.AsString()
}
