// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package npm

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

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
	Repository     Repository                         `json:"repository,omitempty"`
	Author         User                               `json:"author"`
	ReadmeFilename string                             `json:"readmeFilename,omitempty"`
	Users          map[string]bool                    `json:"users,omitempty"`
	License        string                             `json:"license,omitempty"`
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
	License              string              `json:"license,omitempty"`
	Repository           Repository          `json:"repository,omitempty"`
	Keywords             []string            `json:"keywords,omitempty"`
	Dependencies         map[string]string   `json:"dependencies,omitempty"`
	BundleDependencies   []string            `json:"bundleDependencies,omitempty"`
	DevDependencies      map[string]string   `json:"devDependencies,omitempty"`
	PeerDependencies     map[string]string   `json:"peerDependencies,omitempty"`
	Bin                  map[string]string   `json:"bin,omitempty"`
	OptionalDependencies map[string]string   `json:"optionalDependencies,omitempty"`
	Readme               string              `json:"readme,omitempty"`
	Dist                 PackageDistribution `json:"dist"`
	Maintainers          []User              `json:"maintainers,omitempty"`
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

// Repository https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md#version
type Repository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
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

// ParsePackage parses the content into a npm package
func ParsePackage(r io.Reader) (*Package, error) {
	var upload packageUpload
	if err := json.NewDecoder(r).Decode(&upload); err != nil {
		return nil, err
	}

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
				OptionalDependencies:    meta.OptionalDependencies,
				Bin:                     meta.Bin,
				Readme:                  meta.Readme,
				Repository:              meta.Repository,
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

		return p, nil
	}

	return nil, ErrInvalidPackage
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
