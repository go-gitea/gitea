// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package pages contains common Gitea Pages content handling.
package pages

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const (
	// DefaultMaxCompressedSize is the maximum stored gzip size.
	DefaultMaxCompressedSize int64 = 100 * 1024 * 1024
	// DefaultMaxSiteSize is the maximum total size of regular files.
	DefaultMaxSiteSize int64 = 512 * 1024 * 1024
	// DefaultMaxFileSize is the maximum size of one regular file.
	DefaultMaxFileSize int64 = 100 * 1024 * 1024
	// DefaultMaxEntries is the maximum number of archive entries.
	DefaultMaxEntries = 10_000
	// DefaultMaxPathBytes is the maximum path or link target length.
	DefaultMaxPathBytes = 1024
	// DefaultMaxPathDepth is the maximum path depth.
	DefaultMaxPathDepth = 32
	// DefaultMaxSymlinkDepth is the maximum link chain depth.
	DefaultMaxSymlinkDepth = 32
)

var (
	// ErrInvalidArchive reports malformed gzip or TAR data.
	ErrInvalidArchive = errors.New("invalid pages archive")
	// ErrArchiveTooLarge reports a resource limit violation.
	ErrArchiveTooLarge = errors.New("pages archive exceeds its limit")
	// ErrArchivePath reports an unsafe or conflicting path.
	ErrArchivePath = errors.New("invalid pages archive path")
	// ErrArchiveEntry reports an unsupported or conflicting entry.
	ErrArchiveEntry = errors.New("invalid pages archive entry")
	// ErrArchiveSymlink reports an unsafe relative symlink.
	ErrArchiveSymlink = errors.New("invalid pages archive symlink")
	// ErrArchiveSource reports an unreadable source directory.
	ErrArchiveSource = errors.New("invalid pages source directory")
)

// Limits contains the resource bounds shared by Gitea and Pages.
type Limits struct {
	MaxCompressedBytes int64
	MaxTarBytes        int64
	MaxSiteBytes       int64
	MaxFileBytes       int64
	MaxEntries         int
	MaxPathBytes       int
	MaxPathDepth       int
	MaxSymlinkDepth    int
}

// DefaultLimits returns the Pages archive limits from the design.
func DefaultLimits() Limits {
	l := Limits{MaxCompressedBytes: DefaultMaxCompressedSize, MaxSiteBytes: DefaultMaxSiteSize, MaxFileBytes: DefaultMaxFileSize, MaxEntries: DefaultMaxEntries, MaxPathBytes: DefaultMaxPathBytes, MaxPathDepth: DefaultMaxPathDepth, MaxSymlinkDepth: DefaultMaxSymlinkDepth}
	l.MaxTarBytes = l.MaxSiteBytes + int64(l.MaxEntries)*4096 + 1024*1024
	return l
}

func (l Limits) valid() bool {
	return l.MaxCompressedBytes > 0 && l.MaxTarBytes > 0 && l.MaxSiteBytes > 0 && l.MaxFileBytes > 0 && l.MaxEntries > 0 && l.MaxPathBytes > 0 && l.MaxPathDepth > 0 && l.MaxSymlinkDepth > 0
}

type Manifest struct {
	SHA256          [sha256.Size]byte
	SHA256Hex       string
	CompressedBytes int64
	TarBytes        int64
	SiteBytes       int64
	Entries         int
}

type archiveError struct{ kind, err error }

func (e *archiveError) Error() string { return fmt.Sprintf("%s: %v", e.kind, e.err) }
func (e *archiveError) Unwrap() error { return e.kind }
func archiveErrorf(kind error, format string, args ...any) error {
	return &archiveError{kind: kind, err: fmt.Errorf(format, args...)}
}

// ValidateArchive checks one complete gzip stream containing one TAR archive.
func ValidateArchive(ctx context.Context, r io.Reader, limits Limits) (Manifest, error) {
	if !limits.valid() || r == nil {
		return Manifest{}, archiveErrorf(ErrInvalidArchive, "invalid limits or reader")
	}
	hashReader := &hashLimitReader{r: r, hash: sha256.New(), limit: limits.MaxCompressedBytes}
	gz, err := gzip.NewReader(hashReader)
	if err != nil {
		return Manifest{}, archiveErrorf(ErrInvalidArchive, "gzip header: %w", err)
	}
	gz.Multistream(false)
	tarBody := &boundedReader{r: gz, limit: limits.MaxTarBytes + 1}
	tr := tar.NewReader(tarBody)
	entries := make(map[string]archiveEntry)
	folded := make(map[string]string)
	var siteBytes int64
	regularFiles := 0
	count := 0
	for {
		if err := ctx.Err(); err != nil {
			_ = gz.Close()
			return Manifest{}, err
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = gz.Close()
			if errors.Is(err, errReaderLimit) {
				return Manifest{}, archiveErrorf(ErrArchiveTooLarge, "TAR data exceeds %d bytes", limits.MaxTarBytes)
			}
			return Manifest{}, archiveErrorf(ErrInvalidArchive, "TAR: %w", err)
		}
		count++
		if count > limits.MaxEntries {
			_ = gz.Close()
			return Manifest{}, archiveErrorf(ErrArchiveTooLarge, "more than %d entries", limits.MaxEntries)
		}
		name, ignored, err := normalizePath(hdr.Name, limits)
		if err != nil {
			_ = gz.Close()
			return Manifest{}, err
		}
		if ignored {
			if _, err := io.Copy(io.Discard, tr); err != nil {
				_ = gz.Close()
				return Manifest{}, archiveErrorf(ErrInvalidArchive, "root entry: %w", err)
			}
			continue
		}
		if strings.HasSuffix(hdr.Name, "/") && hdr.Typeflag != tar.TypeDir {
			_ = gz.Close()
			return Manifest{}, archiveErrorf(ErrArchiveEntry, "non-directory %q has a trailing slash", name)
		}
		if reservedPath(name) {
			_ = gz.Close()
			return Manifest{}, archiveErrorf(ErrArchivePath, "reserved platform path %q", name)
		}
		key := canonicalPathKey(name)
		if old, ok := folded[key]; ok {
			_ = gz.Close()
			return Manifest{}, archiveErrorf(ErrArchivePath, "path %q conflicts with %q", name, old)
		}
		folded[key] = name
		kind := entryRegular
		switch hdr.Typeflag {
		case tar.TypeReg:
			regularFiles++
			if hdr.Size < 0 || hdr.Size > limits.MaxFileBytes || siteBytes > limits.MaxSiteBytes-hdr.Size {
				_ = gz.Close()
				return Manifest{}, archiveErrorf(ErrArchiveTooLarge, "file %q exceeds content limits", name)
			}
			siteBytes += hdr.Size
		case tar.TypeDir:
			kind = entryDirectory
			if hdr.Size != 0 {
				_ = gz.Close()
				return Manifest{}, archiveErrorf(ErrArchiveEntry, "directory %q has data", name)
			}
		case tar.TypeSymlink:
			kind = entrySymlink
			if hdr.Size != 0 {
				_ = gz.Close()
				return Manifest{}, archiveErrorf(ErrArchiveEntry, "symlink %q has data", name)
			}
			if err := validateLinkTarget(hdr.Linkname, name, limits); err != nil {
				_ = gz.Close()
				return Manifest{}, err
			}
		default:
			_ = gz.Close()
			return Manifest{}, archiveErrorf(ErrArchiveEntry, "unsupported TAR type %d at %q", hdr.Typeflag, name)
		}
		if _, ok := entries[name]; ok {
			_ = gz.Close()
			return Manifest{}, archiveErrorf(ErrArchiveEntry, "duplicate path %q", name)
		}
		entries[name] = archiveEntry{kind: kind, target: hdr.Linkname}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			_ = gz.Close()
			if errors.Is(err, errReaderLimit) {
				return Manifest{}, archiveErrorf(ErrArchiveTooLarge, "TAR data exceeds %d bytes", limits.MaxTarBytes)
			}
			return Manifest{}, archiveErrorf(ErrInvalidArchive, "TAR entry %q: %w", name, err)
		}
	}
	if len(entries) == 0 || regularFiles == 0 {
		_ = gz.Close()
		return Manifest{}, archiveErrorf(ErrArchiveEntry, "archive has no website files")
	}
	if err := validateParentsAndLinks(entries, limits); err != nil {
		_ = gz.Close()
		return Manifest{}, err
	}
	var tail [32 * 1024]byte
	for {
		n, readErr := tarBody.Read(tail[:])
		for _, b := range tail[:n] {
			if b != 0 {
				_ = gz.Close()
				return Manifest{}, archiveErrorf(ErrInvalidArchive, "non-zero data after TAR end")
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = gz.Close()
			return Manifest{}, archiveErrorf(ErrInvalidArchive, "gzip body: %w", readErr)
		}
	}
	if tarBody.n > limits.MaxTarBytes {
		return Manifest{}, archiveErrorf(ErrArchiveTooLarge, "TAR data exceeds %d bytes", limits.MaxTarBytes)
	}
	if err := gz.Close(); err != nil {
		return Manifest{}, archiveErrorf(ErrInvalidArchive, "gzip close: %w", err)
	}
	if hashReader.n > limits.MaxCompressedBytes {
		return Manifest{}, archiveErrorf(ErrArchiveTooLarge, "compressed data exceeds %d bytes", limits.MaxCompressedBytes)
	}
	var extra [1]byte
	n, extraErr := hashReader.r.Read(extra[:])
	if n != 0 || extraErr == nil {
		return Manifest{}, archiveErrorf(ErrInvalidArchive, "data after gzip stream")
	}
	if !errors.Is(extraErr, io.EOF) {
		return Manifest{}, archiveErrorf(ErrInvalidArchive, "reading after gzip stream: %w", extraErr)
	}
	var digest [sha256.Size]byte
	copy(digest[:], hashReader.hash.Sum(nil))
	return Manifest{SHA256: digest, SHA256Hex: hex.EncodeToString(digest[:]), CompressedBytes: hashReader.n, TarBytes: tarBody.n, SiteBytes: siteBytes, Entries: len(entries)}, nil
}

type entryKind uint8

const (
	entryRegular entryKind = iota
	entryDirectory
	entrySymlink
)

type archiveEntry struct {
	kind   entryKind
	target string
}

func reservedPath(name string) bool {
	return name == ".gitea-pages-probe" || strings.HasPrefix(name, ".gitea-pages-probe/")
}

func canonicalPathKey(name string) string {
	return cases.Fold().String(norm.NFC.String(name))
}

func normalizePath(raw string, limits Limits) (string, bool, error) {
	if raw == "" {
		return "", false, archiveErrorf(ErrArchivePath, "empty path")
	}
	if !utf8.ValidString(raw) || strings.ContainsRune(raw, '\x00') || strings.ContainsRune(raw, '\\') || len(raw) > limits.MaxPathBytes || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || windowsVolume(raw) {
		return "", false, archiveErrorf(ErrArchivePath, "path %q is not a relative POSIX path", raw)
	}
	trimmed := strings.TrimRight(raw, "/")
	if trimmed == "" || trimmed == "." {
		return "", true, nil
	}
	parts := strings.Split(trimmed, "/")
	for len(parts) > 0 && parts[0] == "." {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return "", true, nil
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || hasControl(part) {
			return "", false, archiveErrorf(ErrArchivePath, "invalid component in %q", raw)
		}
	}
	if len(parts) > limits.MaxPathDepth {
		return "", false, archiveErrorf(ErrArchivePath, "path %q is too deep", raw)
	}
	name := strings.Join(parts, "/")
	if len(name) > limits.MaxPathBytes {
		return "", false, archiveErrorf(ErrArchivePath, "path %q is too long", raw)
	}
	return name, false, nil
}

func windowsVolume(s string) bool {
	return len(s) >= 2 && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')) && s[1] == ':'
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func validateLinkTarget(target, name string, limits Limits) error {
	if !utf8.ValidString(target) || target == "" || len(target) > limits.MaxPathBytes || strings.ContainsRune(target, '\x00') || strings.ContainsRune(target, '\\') || hasControl(target) || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") || windowsVolume(target) {
		return archiveErrorf(ErrArchiveSymlink, "invalid target %q for %q", target, name)
	}
	return nil
}

func validateParentsAndLinks(entries map[string]archiveEntry, limits Limits) error {
	for name, entry := range entries {
		parts := strings.Split(name, "/")
		for i := 1; i < len(parts); i++ {
			if p, ok := entries[strings.Join(parts[:i], "/")]; ok && p.kind != entryDirectory {
				return archiveErrorf(ErrArchiveEntry, "path %q has non-directory parent", name)
			}
		}
		if entry.kind == entrySymlink {
			if err := resolveLink(name, entries, limits); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveLink(name string, entries map[string]archiveEntry, limits Limits) error {
	link := entries[name]
	pending := []string{}
	if base := path.Dir(name); base != "." {
		pending = append(pending, strings.Split(base, "/")...)
	}
	pending = append(pending, strings.Split(link.target, "/")...)
	resolved := []string{}
	seen := map[string]bool{}
	depth := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(resolved) == 0 {
				return archiveErrorf(ErrArchiveSymlink, "symlink %q leaves root", name)
			}
			resolved = resolved[:len(resolved)-1]
			continue
		default:
			resolved = append(resolved, part)
		}
		if len(resolved) > limits.MaxPathDepth {
			return archiveErrorf(ErrArchiveSymlink, "symlink %q is too deep", name)
		}
		current := strings.Join(resolved, "/")
		entry, ok := entries[current]
		if !ok {
			continue
		}
		if entry.kind == entryRegular && len(pending) > 0 {
			return archiveErrorf(ErrArchiveSymlink, "symlink %q enters a file", name)
		}
		if entry.kind != entrySymlink {
			continue
		}
		depth++
		if depth > limits.MaxSymlinkDepth || seen[current] {
			return archiveErrorf(ErrArchiveSymlink, "symlink %q loops or exceeds depth", name)
		}
		seen[current] = true
		resolved = resolved[:len(resolved)-1]
		pending = append(strings.Split(entry.target, "/"), pending...)
	}
	return nil
}

type hashLimitReader struct {
	r        io.Reader
	hash     hash.Hash
	limit, n int64
}

func (r *hashLimitReader) Read(p []byte) (int, error) {
	if r.n >= r.limit {
		return 0, errReaderLimit
	}
	if int64(len(p)) > r.limit-r.n {
		p = p[:r.limit-r.n]
	}
	n, err := r.r.Read(p)
	if n > 0 {
		r.n += int64(n)
		_, _ = r.hash.Write(p[:n])
	}
	return n, err
}

func (r *hashLimitReader) ReadByte() (byte, error) {
	var b [1]byte
	n, err := r.Read(b[:])
	if n == 1 {
		return b[0], nil
	}
	return 0, err
}

var errReaderLimit = errors.New("reader limit reached")

type boundedReader struct {
	r        io.Reader
	limit, n int64
}

func (r *boundedReader) Read(p []byte) (int, error) {
	if r.n >= r.limit {
		return 0, errReaderLimit
	}
	if int64(len(p)) > r.limit-r.n {
		p = p[:r.limit-r.n]
	}
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

// PackDirectory writes a deterministic TAR.GZ from a local website directory.
// Callers should run Validate on the completed temporary file before storage.
func PackDirectory(ctx context.Context, root string, dst io.Writer, limits Limits) error {
	if !limits.valid() {
		return archiveErrorf(ErrArchiveSource, "invalid limits")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return archiveErrorf(ErrArchiveSource, "source root is not a directory")
	}
	type sourceEntry struct {
		name, full, link string
		info             os.FileInfo
	}
	var source []sourceEntry
	var siteBytes int64
	regularFiles := 0
	folded := make(map[string]string)
	err = filepath.Walk(root, func(full string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if full == root {
			return nil
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return err
		}
		name, ignored, err := normalizePath(filepath.ToSlash(rel), limits)
		if err != nil {
			return err
		}
		if ignored || reservedPath(name) {
			return archiveErrorf(ErrArchivePath, "reserved source path %q", rel)
		}
		item := sourceEntry{name: name, full: full, info: info}
		if info.Mode()&os.ModeSymlink != 0 {
			item.link, err = os.Readlink(full)
			if err != nil {
				return err
			}
			if err := validateLinkTarget(filepath.ToSlash(item.link), name, limits); err != nil {
				return err
			}
		} else if !info.IsDir() && !info.Mode().IsRegular() {
			return archiveErrorf(ErrArchiveEntry, "unsupported source entry %q", name)
		}
		key := canonicalPathKey(name)
		if previous, exists := folded[key]; exists {
			return archiveErrorf(ErrArchivePath, "path %q conflicts with %q", name, previous)
		}
		folded[key] = name
		if info.Mode().IsRegular() {
			regularFiles++
			if info.Size() < 0 || info.Size() > limits.MaxFileBytes || siteBytes > limits.MaxSiteBytes-info.Size() {
				return archiveErrorf(ErrArchiveTooLarge, "file %q exceeds content limits", name)
			}
			siteBytes += info.Size()
		}
		source = append(source, item)
		if len(source) > limits.MaxEntries {
			return archiveErrorf(ErrArchiveTooLarge, "more than %d entries", limits.MaxEntries)
		}
		return nil
	})
	if err != nil {
		return archiveErrorf(ErrArchiveSource, "%w", err)
	}
	sort.Slice(source, func(i, j int) bool { return source[i].name < source[j].name })
	entries := make(map[string]archiveEntry, len(source))
	for _, item := range source {
		kind := entryRegular
		if item.info.IsDir() {
			kind = entryDirectory
		} else if item.info.Mode()&os.ModeSymlink != 0 {
			kind = entrySymlink
		}
		if _, ok := entries[item.name]; ok {
			return archiveErrorf(ErrArchivePath, "duplicate source path %q", item.name)
		}
		entries[item.name] = archiveEntry{kind: kind, target: item.link}
	}
	if len(entries) == 0 || regularFiles == 0 {
		return archiveErrorf(ErrArchiveSource, "source directory is empty")
	}
	if err := validateParentsAndLinks(entries, limits); err != nil {
		return err
	}
	limitedDst := &writeLimit{w: dst, limit: limits.MaxCompressedBytes}
	gz, err := gzip.NewWriterLevel(limitedDst, 6)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(gz)
	for _, item := range source {
		if err := ctx.Err(); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return err
		}
		hdr := &tar.Header{Name: item.name, ModTime: time.Time{}, Uid: 0, Gid: 0, Uname: "", Gname: "", Format: tar.FormatPAX}
		switch {
		case item.info.IsDir():
			hdr.Typeflag, hdr.Mode = tar.TypeDir, 0o755
		case item.info.Mode()&os.ModeSymlink != 0:
			hdr.Typeflag, hdr.Mode, hdr.Linkname = tar.TypeSymlink, 0o644, filepath.ToSlash(item.link)
		case item.info.Mode().IsRegular():
			hdr.Typeflag, hdr.Mode, hdr.Size = tar.TypeReg, 0o644, item.info.Size()
			if hdr.Size > limits.MaxFileBytes {
				_ = tw.Close()
				_ = gz.Close()
				return archiveErrorf(ErrArchiveTooLarge, "file %q exceeds limit", item.name)
			}
		default:
			_ = tw.Close()
			_ = gz.Close()
			return archiveErrorf(ErrArchiveEntry, "unsupported source entry %q", item.name)
		}
		if err := tw.WriteHeader(hdr); err != nil {
			_ = gz.Close()
			return err
		}
		if hdr.Typeflag == tar.TypeReg {
			f, err := os.Open(item.full)
			if err != nil {
				_ = tw.Close()
				_ = gz.Close()
				return err
			}
			_, copyErr := io.CopyN(tw, f, hdr.Size)
			closeErr := f.Close()
			if copyErr != nil {
				_ = tw.Close()
				_ = gz.Close()
				return copyErr
			}
			if closeErr != nil {
				_ = tw.Close()
				_ = gz.Close()
				return closeErr
			}
		}
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		if errors.Is(err, errWriterLimit) {
			return archiveErrorf(ErrArchiveTooLarge, "compressed data exceeds %d bytes", limits.MaxCompressedBytes)
		}
		return err
	}
	if err := gz.Close(); err != nil {
		if errors.Is(err, errWriterLimit) {
			return archiveErrorf(ErrArchiveTooLarge, "compressed data exceeds %d bytes", limits.MaxCompressedBytes)
		}
		return err
	}
	return nil
}

var errWriterLimit = errors.New("writer limit reached")

type writeLimit struct {
	w     io.Writer
	limit int64
	n     int64
}

func (w *writeLimit) Write(p []byte) (int, error) {
	if w.n >= w.limit {
		return 0, errWriterLimit
	}
	if int64(len(p)) > w.limit-w.n {
		p = p[:w.limit-w.n]
	}
	n, err := w.w.Write(p)
	w.n += int64(n)
	if err == nil && n < len(p) {
		return n, io.ErrShortWrite
	}
	if err != nil {
		return n, err
	}
	return n, nil
}
