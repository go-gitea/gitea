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
	ErrUnsafeArchivePath     = errors.New("module archive contains an unsafe file path")
	ErrUnsafeArchiveLink     = errors.New("module archive contains a link pointing outside the archive")
	ErrEmptyModule           = errors.New("module archive contains no .tf files")
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

	// tarHeaderSize is the on-the-wire size of a tar entry header. It is
	// charged against the byte ceiling so empty entries still cost budget.
	tarHeaderSize = 512

	// maxIndexedDirDepth is the deepest directory level read for metadata:
	// `<wrapper>/modules/<name>` is three components.
	maxIndexedDirDepth = 3
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
	// RootDir is the single top-level directory the module was wrapped in
	// (e.g. a GitHub release tarball), or "" when the module sits at the
	// archive root. It is transient parse state — not persisted — used by
	// the upload handler to normalize a wrapped archive to a flat one via
	// NormalizeArchive.
	RootDir string
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

// reservedModuleDirs are the standard-module-structure directory names
// that must never be mistaken for an archive wrapper directory.
// See https://developer.hashicorp.com/terraform/language/modules/develop/structure
var reservedModuleDirs = map[string]struct{}{"modules": {}, "examples": {}}

// dirFiles holds the parse-relevant files collected for a single
// directory level of the archive (the root, or a top-level directory).
type dirFiles struct {
	tf     map[string][]byte // basename -> .tf source
	readme string
}

// ParseModuleArchive consumes a gzipped tar archive and extracts the root
// module's metadata. The module sources may sit either at the archive
// root (`tar -czf module.tgz *`) or wrapped in a single top-level
// directory (a GitHub release tarball, `git archive --prefix`, ...); the
// wrapper is reported via Module.RootDir so the upload handler can
// normalize it away. The archive only needs to contain at least one .tf
// file somewhere — a collection of submodules with no root module is
// valid and yields empty root metadata rather than an error.
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
		// topDirs is the set of distinct top-level directory names and
		// topLevelFile records whether any file sits at the archive root;
		// together they detect the single-wrapper-directory layout.
		topDirs      = map[string]struct{}{}
		topLevelFile bool
		// Presence of any .tf / .tf.json anywhere decides whether the
		// archive is a Terraform module at all.
		tfAnywhere, tfJSONAnywhere bool
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
	handleFile := func(dir, base string, size int64) error {
		lower := strings.ToLower(base)
		switch {
		case strings.HasSuffix(lower, ".tf"):
			data, n, err := readCapped(tr, size, maxSize, consumed)
			if err != nil {
				return err
			}
			consumed += n
			dirEntry(dir).tf[base] = data
		case lower == "readme.md" || lower == "readme":
			data, n, err := readCapped(tr, size, maxSize, consumed)
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

	var entries int
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}

		// Charge the header itself so an archive of empty entries still
		// exhausts the byte budget, and bound the entry count outright.
		entries++
		if entries > maxArchiveEntries {
			return nil, ErrTooManyArchiveEntries
		}
		consumed += tarHeaderSize
		if consumed > maxSize {
			return nil, ErrArchiveTooLarge
		}

		// Reject absolute and traversing paths before we touch the file.
		clean := path.Clean(hdr.Name)
		if path.IsAbs(clean) || strings.HasPrefix(clean, "../") || clean == ".." {
			return nil, ErrUnsafeArchivePath
		}

		// Links are stored verbatim and get materialized when the consumer
		// unpacks the archive, so a link escaping the archive must never be
		// published.
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			if !isSafeLink(clean, hdr.Linkname, hdr.Typeflag == tar.TypeSymlink) {
				return nil, ErrUnsafeArchiveLink
			}
			continue
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

		base := path.Base(clean)
		switch lower := strings.ToLower(base); {
		case strings.HasSuffix(lower, ".tf.json"):
			tfJSONAnywhere = true
		case strings.HasSuffix(lower, ".tf"):
			tfAnywhere = true
		}

		// Read metadata for the root module and for `modules/<name>`
		// submodules; anything deeper is an example or a nested detail and
		// is skipped.
		dir := path.Dir(clean)
		if dir == "." {
			dir = ""
			topLevelFile = true
		}
		if dirDepth(dir) > maxIndexedDirDepth {
			n, err := skipCapped(tr, maxSize, consumed)
			if err != nil {
				return nil, err
			}
			consumed += n
			continue
		}
		if err := handleFile(dir, base, hdr.Size); err != nil {
			return nil, err
		}
	}

	if !tfAnywhere {
		if tfJSONAnywhere {
			return nil, ErrUnsupportedTFFormat
		}
		return nil, ErrEmptyModule
	}

	moduleDir := wrapperDir(topDirs, topLevelFile)
	df := byDir[moduleDir]
	if df == nil {
		df = &dirFiles{} // collection with no root module: empty root metadata
	}

	root, description, err := parseRoot(df.tf)
	if err != nil {
		return nil, err
	}

	submodules, err := parseSubmodules(byDir, moduleDir)
	if err != nil {
		return nil, err
	}

	return &Module{
		Metadata: &Metadata{
			Description: description,
			Readme:      df.readme,
			Root:        root,
			Providers:   root.Providers,
			Submodules:  submodules,
		},
		RootDir: moduleDir,
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

// isSafeLink reports whether a tar link entry resolves to a location
// inside the archive. Hard link targets are archive-root relative; symlink
// targets resolve against the link's own directory. Absolute targets are
// always rejected.
func isSafeLink(name, linkname string, symlink bool) bool {
	if linkname == "" || path.IsAbs(linkname) || strings.HasPrefix(linkname, "/") {
		return false
	}
	target := linkname
	if symlink {
		target = path.Join(path.Dir(name), linkname)
	}
	target = path.Clean(target)
	return target != ".." && !strings.HasPrefix(target, "../")
}

// parseSubmodules extracts metadata for each `modules/<name>` directory of
// the standard module structure, so a module made only of submodules still
// has something to show.
func parseSubmodules(byDir map[string]*dirFiles, rootDir string) ([]*Submodule, error) {
	prefix := "modules/"
	if rootDir != "" {
		prefix = rootDir + "/modules/"
	}

	names := make([]string, 0, len(byDir))
	for dir := range byDir {
		name, ok := strings.CutPrefix(dir, prefix)
		if !ok || name == "" || strings.Contains(name, "/") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	submodules := make([]*Submodule, 0, len(names))
	for _, name := range names {
		df := byDir[prefix+name]
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

// wrapperDir returns the single top-level directory that wraps the whole
// archive (a GitHub release tarball, `git archive --prefix`, ...), or ""
// when the module already sits at the archive root. A wrapper exists only
// when every entry lives under exactly one top-level directory whose name
// is not a reserved standard-structure directory (so a collection whose
// sole top-level entry is `modules/` is not mistaken for a wrapper).
func wrapperDir(topDirs map[string]struct{}, topLevelFile bool) string {
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

// NormalizeArchive rewrites a gzipped tar so that the contents of the
// single wrapper directory rootDir become the archive root, dropping the
// wrapper (and any stray entries outside it). The result is a flat
// archive that the registry stores and serves verbatim, so the download
// path never needs a go-getter subdir. The total decompressed size is
// capped at maxParseSize as a safety net (the archive has already passed
// the same ceiling during parsing).
func NormalizeArchive(dst io.Writer, src io.Reader, rootDir string) (err error) {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("invalid gzip stream: %w", err)
	}
	defer gzr.Close()

	gzw := gzip.NewWriter(dst)
	tw := tar.NewWriter(gzw)
	// Both writers must be closed on every path: tar.Writer.Close flushes
	// the trailer and gzip.Writer.Close the stream footer, so an early
	// return would otherwise emit a truncated archive.
	defer func() {
		cerr := tw.Close()
		if gzerr := gzw.Close(); cerr == nil {
			cerr = gzerr
		}
		if err == nil {
			err = cerr
		}
	}()

	tr := tar.NewReader(gzr)
	prefix := rootDir + "/"

	var written int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		clean := path.Clean(hdr.Name)
		if clean == rootDir {
			continue // the wrapper directory entry itself
		}
		rel := strings.TrimPrefix(clean, prefix)
		if rel == clean {
			continue // entry outside the wrapper directory
		}
		if hdr.Typeflag == tar.TypeDir {
			rel += "/"
		}

		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		n, err := io.Copy(tw, io.LimitReader(tr, maxParseSize-written+1))
		if err != nil {
			return err
		}
		written += n
		if written > maxParseSize {
			return ErrArchiveTooLarge
		}
	}

	return nil
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

// readCapped reads the current tar entry into memory, allocating exactly
// the size declared by its header so an entry with no payload costs no
// allocation at all. size is the header's declared length; the read is
// rejected when it would push the running total past maxSize.
func readCapped(tr *tar.Reader, size, maxSize, consumed int64) ([]byte, int64, error) {
	remaining := maxSize - consumed
	if remaining <= 0 || size > remaining {
		return nil, 0, ErrArchiveTooLarge
	}
	if size <= 0 {
		return nil, 0, nil
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(tr, data); err != nil {
		return nil, 0, err
	}
	return data, size, nil
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
