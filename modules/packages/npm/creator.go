// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"

	"github.com/hashicorp/go-version"
)

var (
	// ErrInvalidPackage indicates an invalid package
	ErrInvalidPackage = util.NewInvalidArgumentErrorf("package is invalid")
	// ErrInvalidPackageName indicates an invalid name
	ErrInvalidPackageName = util.NewInvalidArgumentErrorf("package name is invalid")
	// ErrInvalidPackageVersion indicates an invalid version
	ErrInvalidPackageVersion = util.NewInvalidArgumentErrorf("package version is invalid")
	// ErrInvalidAttachment indicates a invalid attachment
	ErrInvalidAttachment = util.NewInvalidArgumentErrorf("package attachment is invalid")
	// ErrInvalidIntegrity indicates an integrity validation error
	ErrInvalidIntegrity = util.NewInvalidArgumentErrorf("failed to validate integrity")
)

var nameMatch = regexp.MustCompile(`^(@[a-z0-9-][a-z0-9-._]*/)?[a-z0-9-][a-z0-9-._]*$`)

// Package represents a npm package
type Package struct {
	Name     string
	Version  string
	DistTags []string
	Metadata Metadata
	Filename string
	Data     []byte
}

// PackageMetadata https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md#package
type PackageMetadata struct {
	ID             string                             `json:"_id"`
	Name           string                             `json:"name"`
	Description    string                             `json:"description"`
	DistTags       map[string]string                  `json:"dist-tags,omitempty"`
	Versions       map[string]*PackageMetadataVersion `json:"versions"`
	Readme         string                             `json:"readme,omitempty"`
	Maintainers    []User                             `json:"maintainers,omitempty"`
	Time           map[string]time.Time               `json:"time,omitempty"`
	Homepage       string                             `json:"homepage,omitempty"`
	Keywords       []string                           `json:"keywords,omitempty"`
	Repository     Repository                         `json:"repository"`
	Author         User                               `json:"author"`
	ReadmeFilename string                             `json:"readmeFilename,omitempty"`
	Users          map[string]bool                    `json:"users,omitempty"`
	License        License                            `json:"license,omitempty"`
}

type License string

func (l *License) UnmarshalJSON(data []byte) error {
	switch data[0] {
	case '"':
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*l = License(value)
	case '{':
		var values map[string]any
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
		value, _ := values["type"].(string)
		*l = License(value)
	}
	return nil
}

// PackageMetadataVersion documentation: https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md#version
// PackageMetadataVersion response: https://github.com/npm/registry/blob/master/docs/responses/package-metadata.md#abbreviated-version-object
type PackageMetadataVersion struct {
	ID                   string              `json:"_id"`
	Name                 string              `json:"name"`
	Version              string              `json:"version"`
	Description          string              `json:"description"`
	Author               User                `json:"author"`
	Homepage             string              `json:"homepage,omitempty"`
	License              License             `json:"license,omitempty"`
	Repository           Repository          `json:"repository"`
	Keywords             []string            `json:"keywords,omitempty"`
	Dependencies         map[string]string   `json:"dependencies,omitempty"`
	BundleDependencies   []string            `json:"bundleDependencies,omitempty"`
	DevDependencies      map[string]string   `json:"devDependencies,omitempty"`
	PeerDependencies     map[string]string   `json:"peerDependencies,omitempty"`
	PeerDependenciesMeta map[string]any      `json:"peerDependenciesMeta,omitempty"`
	Bin                  Bin                 `json:"bin,omitempty"`
	OptionalDependencies map[string]string   `json:"optionalDependencies,omitempty"`
	Readme               string              `json:"readme,omitempty"`
	Dist                 PackageDistribution `json:"dist"`
	Maintainers          []User              `json:"maintainers,omitempty"`
	HasInstallScript     bool                `json:"hasInstallScript,omitempty"`
	HasShrinkwrap        bool                `json:"_hasShrinkwrap,omitempty"`
	Engines              map[string]string   `json:"engines,omitempty"`
	CPU                  []string            `json:"cpu,omitempty"`
	OS                   []string            `json:"os,omitempty"`
	Directories          map[string]string   `json:"directories,omitempty"`
	Funding              any                 `json:"funding,omitempty"`
	AcceptDependencies   map[string]string   `json:"acceptDependencies,omitempty"`
	Deprecated           string              `json:"deprecated,omitempty"`
}

// PackageDistribution https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md#version
type PackageDistribution struct {
	Integrity    string `json:"integrity"`
	Shasum       string `json:"shasum"`
	Tarball      string `json:"tarball"`
	FileCount    int    `json:"fileCount,omitempty"`
	UnpackedSize int    `json:"unpackedSize,omitempty"`
	NpmSignature string `json:"npm-signature,omitempty"`
}

type PackageSearch struct {
	Objects []*PackageSearchObject `json:"objects"`
	Total   int64                  `json:"total"`
}

type PackageSearchObject struct {
	Package *PackageSearchPackage `json:"package"`
}

type PackageSearchPackage struct {
	Scope       string                     `json:"scope"`
	Name        string                     `json:"name"`
	Version     string                     `json:"version"`
	Date        time.Time                  `json:"date"`
	Description string                     `json:"description"`
	Author      User                       `json:"author"`
	Publisher   User                       `json:"publisher"`
	Maintainers []User                     `json:"maintainers"`
	Keywords    []string                   `json:"keywords,omitempty"`
	Links       *PackageSearchPackageLinks `json:"links"`
}

type PackageSearchPackageLinks struct {
	Registry   string `json:"npm"`
	Homepage   string `json:"homepage,omitempty"`
	Repository string `json:"repository,omitempty"`
}

// User https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md#package
type User struct {
	Username string `json:"username,omitempty"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	URL      string `json:"url,omitempty"`
}

// UnmarshalJSON is needed because User objects can be strings or objects
func (u *User) UnmarshalJSON(data []byte) error {
	switch data[0] {
	case '"':
		if err := json.Unmarshal(data, &u.Name); err != nil {
			return err
		}
	case '{':
		var tmp struct {
			Username string `json:"username"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			URL      string `json:"url"`
		}
		if err := json.Unmarshal(data, &tmp); err != nil {
			return err
		}
		u.Username = tmp.Username
		u.Name = tmp.Name
		u.Email = tmp.Email
		u.URL = tmp.URL
	}
	return nil
}

// Repository https://docs.npmjs.com/cli/v11/configuring-npm/package-json#repository
type Repository struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Directory string `json:"directory,omitempty"`
}

// UnmarshalJSON is needed because the repository field can be a string or an object.
func (r *Repository) UnmarshalJSON(data []byte) error {
	switch data[0] {
	case '"':
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		r.URL = value
	case '{':
		type repositoryAlias Repository // avoid recursion into this method
		var value repositoryAlias
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*r = Repository(value)
	}
	return nil
}

// Bin maps command names to executable files. npm also allows a single string,
// in which case the command is named after the package (resolved in ParsePackage).
type Bin map[string]string

// UnmarshalJSON is needed because the bin field can be a string or an object.
func (b *Bin) UnmarshalJSON(data []byte) error {
	switch data[0] {
	case '"':
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*b = Bin{"": value}
	case '{':
		var value map[string]string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*b = value
	}
	return nil
}

// PackageAttachment https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md#package
type PackageAttachment struct {
	ContentType string `json:"content_type"`
	Data        string `json:"data"`
	Length      int    `json:"length"`
}

type packageUpload struct {
	PackageMetadata
	Attachments map[string]*PackageAttachment `json:"_attachments"`
}

// ParseUpload decodes an npm PUT body. Exactly one of the returned pointers
// is non-nil on success; a body without `_attachments` is a deprecate request,
// otherwise it is a publish.
func ParseUpload(r io.Reader) (*Package, *PackageDeprecation, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	var upload packageUpload
	if err := json.Unmarshal(body, &upload); err != nil {
		return nil, nil, err
	}
	if len(upload.Attachments) == 0 {
		dep, err := parseUploadDeprecation(&upload, body)
		return nil, dep, err
	}
	p, err := parseUploadPackage(&upload)
	return p, nil, err
}

// ParsePackage parses an npm publish PUT body. Deprecate bodies are rejected.
func ParsePackage(r io.Reader) (*Package, error) {
	p, _, err := ParseUpload(r)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrInvalidAttachment
	}
	return p, nil
}

// parseUploadPackage builds a Package from a decoded publish body.
func parseUploadPackage(upload *packageUpload) (*Package, error) {
	for _, meta := range upload.Versions {
		if !validateName(meta.Name) {
			return nil, ErrInvalidPackageName
		}

		v, err := version.NewSemver(meta.Version)
		if err != nil {
			return nil, ErrInvalidPackageVersion
		}

		scope := ""
		name := meta.Name
		nameParts := strings.SplitN(meta.Name, "/", 2)
		if len(nameParts) == 2 {
			scope = nameParts[0]
			name = nameParts[1]
		}

		if !validation.IsValidURL(meta.Homepage) {
			meta.Homepage = ""
		}

		// A string "bin" means a single executable named after the package.
		if cmd, ok := meta.Bin[""]; ok && len(meta.Bin) == 1 {
			meta.Bin = Bin{name: cmd}
		}

		p := &Package{
			Name:     meta.Name,
			Version:  v.String(),
			DistTags: make([]string, 0, 1),
			Metadata: Metadata{
				Scope:                   scope,
				Name:                    name,
				Description:             meta.Description,
				Author:                  meta.Author.Name,
				License:                 meta.License,
				ProjectURL:              meta.Homepage,
				Keywords:                meta.Keywords,
				Dependencies:            meta.Dependencies,
				BundleDependencies:      meta.BundleDependencies,
				DevelopmentDependencies: meta.DevDependencies,
				PeerDependencies:        meta.PeerDependencies,
				PeerDependenciesMeta:    meta.PeerDependenciesMeta,
				OptionalDependencies:    meta.OptionalDependencies,
				Bin:                     meta.Bin,
				Readme:                  meta.Readme,
				Repository:              meta.Repository,
				Engines:                 meta.Engines,
				CPU:                     meta.CPU,
				OS:                      meta.OS,
				Directories:             meta.Directories,
				Funding:                 meta.Funding,
				AcceptDependencies:      meta.AcceptDependencies,
				Deprecated:              meta.Deprecated,
			},
		}

		for tag := range upload.DistTags {
			p.DistTags = append(p.DistTags, tag)
		}

		p.Filename = strings.ToLower(fmt.Sprintf("%s-%s.tgz", name, p.Version))

		attachment := func() *PackageAttachment {
			for _, a := range upload.Attachments {
				return a
			}
			return nil
		}()
		if attachment == nil || len(attachment.Data) == 0 {
			return nil, ErrInvalidAttachment
		}

		data, err := base64.StdEncoding.DecodeString(attachment.Data)
		if err != nil {
			return nil, ErrInvalidAttachment
		}
		p.Data = data

		integrity := strings.SplitN(meta.Dist.Integrity, "-", 2)
		if len(integrity) != 2 {
			return nil, ErrInvalidIntegrity
		}
		integrityHash, err := base64.StdEncoding.DecodeString(integrity[1])
		if err != nil {
			return nil, ErrInvalidIntegrity
		}
		var hash []byte
		switch integrity[0] {
		case "sha1":
			tmp := sha1.Sum(data)
			hash = tmp[:]
		case "sha512":
			tmp := sha512.Sum512(data)
			hash = tmp[:]
		}
		if !bytes.Equal(integrityHash, hash) {
			return nil, ErrInvalidIntegrity
		}

		// Derive _hasShrinkwrap and hasInstallScript from the tarball; the
		// packument can lie about either.
		p.Metadata.HasShrinkwrap, p.Metadata.HasInstallScript = inspectTarball(data)

		return p, nil
	}

	return nil, ErrInvalidPackage
}

// maxNpmTarballScanBytes caps the decompressed tarball bytes inspectTarball
// will read; defends against gzip bombs.
const maxNpmTarballScanBytes = int64(32 * 1024 * 1024) // 32 MiB

// maxNpmPackageJSONBytes caps the package.json bytes decoded from the tarball.
const maxNpmPackageJSONBytes = int64(1 * 1024 * 1024) // 1 MiB

// inspectTarball reports hasShrinkwrap (presence of package/npm-shrinkwrap.json)
// and hasInstallScript (package/package.json declares any of preinstall,
// install, postinstall). Both must be derived server-side because the client
// can lie in the packument. Any read/decode error yields (false, false) so a
// malformed archive does not block publishing.
func inspectTarball(data []byte) (hasShrinkwrap, hasInstallScript bool) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return false, false
	}
	defer gr.Close()

	tr := tar.NewReader(io.LimitReader(gr, maxNpmTarballScanBytes))
	for {
		hdr, err := tr.Next()
		if err != nil {
			return
		}
		// npm pack puts files under a single root directory (usually "package/").
		name := strings.TrimPrefix(hdr.Name, "./")
		if strings.Count(name, "/") != 1 {
			continue
		}
		switch {
		case strings.HasSuffix(name, "/npm-shrinkwrap.json"):
			hasShrinkwrap = true
		case strings.HasSuffix(name, "/package.json"):
			hasInstallScript = tarballDeclaresInstallScript(tr)
		}
		if hasShrinkwrap && hasInstallScript {
			return
		}
	}
}

// tarballDeclaresInstallScript reports whether a package.json declares any
// of preinstall, install, postinstall.
func tarballDeclaresInstallScript(r io.Reader) bool {
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.NewDecoder(io.LimitReader(r, maxNpmPackageJSONBytes)).Decode(&pkg); err != nil {
		return false
	}
	for _, name := range []string{"preinstall", "install", "postinstall"} {
		if strings.TrimSpace(pkg.Scripts[name]) != "" {
			return true
		}
	}
	return false
}

func validateName(name string) bool {
	if strings.TrimSpace(name) != name {
		return false
	}
	if len(name) == 0 || len(name) > 214 {
		return false
	}
	return nameMatch.MatchString(name)
}

// PackageDeprecation is the result of parsing an npm deprecate request body.
// Versions maps a version string to its deprecation message; an empty message
// means "undeprecate".
type PackageDeprecation struct {
	PackageName string
	Versions    map[string]string
}

// ParsePackageDeprecation parses an npm deprecate PUT body. The npm CLI sends
// the full package document (no `_attachments`) with the `deprecated` string
// field set or cleared on each affected version.
func ParsePackageDeprecation(r io.Reader) (*PackageDeprecation, error) {
	_, dep, err := ParseUpload(r)
	if err != nil {
		return nil, err
	}
	if dep == nil {
		return nil, ErrInvalidPackage
	}
	return dep, nil
}

// parseUploadDeprecation builds a PackageDeprecation from a body with no
// `_attachments`. Only versions whose object explicitly contained a
// `deprecated` key are emitted, so a subset PUT cannot silently undeprecate
// versions it omitted. An empty string still means "undeprecate".
func parseUploadDeprecation(upload *packageUpload, body []byte) (*PackageDeprecation, error) {
	if !validateName(upload.Name) {
		return nil, ErrInvalidPackageName
	}
	var raw struct {
		Versions map[string]map[string]any `json:"versions"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	d := &PackageDeprecation{
		PackageName: upload.Name,
		Versions:    make(map[string]string, len(raw.Versions)),
	}
	for v, meta := range upload.Versions {
		if meta == nil {
			continue
		}
		if _, ok := raw.Versions[v]["deprecated"]; !ok {
			continue
		}
		d.Versions[v] = meta.Deprecated
	}
	return d, nil
}
