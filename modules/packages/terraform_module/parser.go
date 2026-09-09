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

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Validation errors returned by the parser.
var (
	ErrInvalidName           = errors.New("module name is invalid")
	ErrInvalidProvider       = errors.New("module provider is invalid")
	ErrInvalidVersion        = errors.New("module version is invalid")
	ErrArchiveTooLarge       = errors.New("module archive exceeds size limit")
	ErrTooManyArchiveEntries = errors.New("module archive contains too many entries")
	ErrNoRootModule          = errors.New("no terraform module at the archive root: package the module directory's contents, not the directory itself (e.g. `tar -czf module.tar.gz -C path/to/module .`)")
	ErrUnsupportedTFFormat   = errors.New("only .tf files are supported (.tf.json is not parsed in v1)")
)

const (
	// maxParseSize is a hard ceiling on the total decompressed bytes read
	// while parsing an archive, independent of the configurable storage
	// quota (LIMIT_SIZE_TERRAFORM_MODULE). It guards against gzip bombs even
	// when the operator disables the storage quota with -1. Real Terraform
	// modules are KB-scale, so 32 MiB is generous.
	maxParseSize = 32 << 20 // 32 MiB

	// maxArchiveEntries caps how many tar entries are examined. Entry
	// headers carry no payload, so without this an archive of nothing but
	// empty headers would compress to almost nothing yet cost one
	// allocation per entry, bypassing the byte ceiling entirely.
	maxArchiveEntries = 32 << 10 // 32768

	// tarBlockSize is the tar on-the-wire unit: every entry header is one
	// block and payloads are padded up to a block boundary.
	tarBlockSize = 512

	// maxIndexedDirDepth is the deepest directory level read for metadata:
	// `modules/<name>` is two components.
	maxIndexedDirDepth = 2

	// submodulesDir is the standard-module-structure directory holding
	// nested modules; only it and the archive root are read for metadata.
	// See https://developer.hashicorp.com/terraform/language/modules/develop/structure
	submodulesDir = "modules/"
)

// NormalizeVersion validates a module version and returns its canonical
// semver form. Terraform accepts both `v1.0.0` and `1.0.0`; normalizing on
// the way in keeps a single naming scheme in the registry instead of
// storing some versions prefixed and others not.
func NormalizeVersion(s string) (string, error) {
	v, err := version.NewSemver(s)
	if err != nil {
		return "", ErrInvalidVersion
	}
	return v.String(), nil
}

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

// reservedModuleDirs are standard-module-structure directory names. A lone
// top-level `modules/` is a collection, not an archive wrapped in a
// release directory, so these never trigger the "wrapped" diagnostic.
var reservedModuleDirs = map[string]struct{}{"modules": {}, "examples": {}}

// dirFiles holds the parse-relevant files collected for one directory of
// the archive (the root, or a `modules/<name>` submodule).
type dirFiles struct {
	tf     map[string][]byte // basename -> .tf source
	readme string
}

// ParseModuleArchive reads a gzipped tar archive and extracts metadata for
// the root module and its `modules/<name>` submodules. The archive is never
// modified — it is stored and served exactly as uploaded — so the module
// has to sit at the archive root: a collection of submodules with no root
// module is fine, while a module wrapped in a top-level directory (a
// GitHub release tarball) is rejected, since nothing at its root would be
// consumable.
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
		// byDir maps a directory ("" for the archive root) to its files.
		byDir = map[string]*dirFiles{}
		// topDirs and topLevelFile only feed the diagnostic for an archive
		// wrapped in a single top-level directory.
		topDirs      = map[string]struct{}{}
		topLevelFile bool
		tfJSONSeen   bool // a .tf.json in an indexed directory
	)

	dirEntry := func(dir string) *dirFiles {
		df := byDir[dir]
		if df == nil {
			df = &dirFiles{tf: map[string][]byte{}}
			byDir[dir] = df
		}
		return df
	}

	// handleFile keeps .tf and README payloads for the given directory;
	// anything else is left for the next tr.Next to skip. The entry's
	// size has already been charged against the ceiling.
	handleFile := func(dir, base string, size int64) error {
		lower := strings.ToLower(base)
		switch {
		case strings.HasSuffix(lower, ".tf.json"):
			tfJSONSeen = true
		case strings.HasSuffix(lower, ".tf"):
			data, err := readEntry(tr, size)
			if err != nil {
				return err
			}
			dirEntry(dir).tf[base] = data
		case lower == "readme.md" || lower == "readme":
			data, err := readEntry(tr, size)
			if err != nil {
				return err
			}
			dirEntry(dir).readme = string(data)
		}
		return nil
	}

	var entries int
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}

		// Charge each entry's full on-the-wire size before reading anything
		// so the byte ceiling is strict and an archive of empty entries
		// still exhausts it; bound the entry count outright as well.
		entries++
		if entries > maxArchiveEntries {
			return nil, ErrTooManyArchiveEntries
		}
		consumed += tarWireSize(hdr.Size)
		if consumed > maxSize {
			return nil, ErrArchiveTooLarge
		}

		clean := path.Clean(hdr.Name)
		if clean == "." || isArchiveJunk(clean) {
			continue // unread payload is skipped by the next tr.Next
		}

		if strings.Contains(clean, "/") {
			topDirs[clean[:strings.IndexByte(clean, '/')]] = struct{}{}
		} else if hdr.Typeflag == tar.TypeDir {
			topDirs[clean] = struct{}{}
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		dir := path.Dir(clean)
		if dir == "." {
			dir = ""
			topLevelFile = true
		}
		if dirDepth(dir) > maxIndexedDirDepth {
			continue
		}
		if err := handleFile(dir, path.Base(clean), hdr.Size); err != nil {
			return nil, err
		}
	}

	rootFiles := byDir[""]
	if rootFiles == nil {
		rootFiles = &dirFiles{}
	}
	submodules, err := parseSubmodules(byDir)
	if err != nil {
		return nil, err
	}

	// Verbatim storage means only what sits at the root is consumable.
	if len(rootFiles.tf) == 0 && len(submodules) == 0 {
		if dir := wrappedIn(topDirs, topLevelFile); dir != "" {
			return nil, fmt.Errorf("%w (the archive is wrapped in %q)", ErrNoRootModule, dir)
		}
		if tfJSONSeen {
			return nil, ErrUnsupportedTFFormat
		}
		return nil, ErrNoRootModule
	}

	root, description, err := parseRoot(rootFiles.tf)
	if err != nil {
		return nil, err
	}

	return &Module{
		Metadata: &Metadata{
			Description: description,
			Readme:      rootFiles.readme,
			Root:        root,
			Providers:   root.Providers,
			Submodules:  submodules,
		},
	}, nil
}

// dirDepth returns the number of path components in a cleaned directory
// path ("" is the archive root, depth 0).
func dirDepth(dir string) int {
	if dir == "" {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// wrappedIn returns the single top-level directory every entry lives under
// (typically a GitHub release tarball's `repo-version/`), or "" when files
// sit at the root or the sole directory is a standard-structure one. Used
// only to make the rejection message name the culprit.
func wrappedIn(topDirs map[string]struct{}, topLevelFile bool) string {
	if topLevelFile || len(topDirs) != 1 {
		return ""
	}
	var only string
	for d := range topDirs {
		only = d
	}
	if _, reserved := reservedModuleDirs[only]; reserved {
		return ""
	}
	return only
}

// parseSubmodules extracts metadata for each `modules/<name>` directory of
// the standard module structure, so a module made only of submodules still
// has something to show.
func parseSubmodules(byDir map[string]*dirFiles) ([]*Submodule, error) {
	names := make([]string, 0, len(byDir))
	for dir := range byDir {
		name, ok := strings.CutPrefix(dir, submodulesDir)
		if !ok || name == "" || strings.Contains(name, "/") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	submodules := make([]*Submodule, 0, len(names))
	for _, name := range names {
		df := byDir[submodulesDir+name]
		if len(df.tf) == 0 {
			continue
		}
		root, description, err := parseRoot(df.tf)
		if err != nil {
			return nil, err
		}
		submodules = append(submodules, &Submodule{
			Name:        name,
			Description: description,
			Readme:      df.readme,
			Root:        root,
		})
	}
	if len(submodules) == 0 {
		return nil, nil
	}
	return submodules, nil
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

// tarWireSize returns the bytes an entry occupies in the tar stream: one
// header block plus its payload padded to a whole block. This is what the
// byte ceiling must charge, because tar.Reader consumes the padding
// silently. Absurd declared sizes are clamped past any ceiling instead of
// being allowed to overflow the arithmetic.
func tarWireSize(payload int64) int64 {
	if payload < 0 || payload > maxParseSize {
		return maxParseSize + 1
	}
	return tarBlockSize + (payload+tarBlockSize-1)/tarBlockSize*tarBlockSize
}

// readEntry reads the current tar entry into memory, allocating exactly
// the size declared by its header so an entry with no payload costs no
// allocation at all. The caller has already charged size against the
// byte ceiling.
func readEntry(tr *tar.Reader, size int64) ([]byte, error) {
	if size <= 0 {
		return nil, nil
	}
	data := make([]byte, int(size))
	if _, err := io.ReadFull(tr, data); err != nil {
		return nil, err
	}
	return data, nil
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
