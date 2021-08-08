// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package npm

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/validation"

	"github.com/hashicorp/go-version"
)

var (
	// ErrInvalidPackage indicates an invalid package
	ErrInvalidPackage = errors.New("The package is invalid")
	// ErrInvalidPackageName indicates an invalid name
	ErrInvalidPackageName = errors.New("The package name is invalid")
	// ErrInvalidPackageVersion indicates an invalid version
	ErrInvalidPackageVersion = errors.New("The package version is invalid")
	// ErrInvalidAttachment indicates a invalid attachment
	ErrInvalidAttachment = errors.New("The package attachment is invalid")
	// ErrInvalidIntegrity indicates an integrity validation error
	ErrInvalidIntegrity = errors.New("Failed to validate integrity")
)

var nameMatch = regexp.MustCompile(`\A(@[^\/~'!\(\)\*]+?)[\/]([^_.][^\/~'!\(\)\*]+)\z`)

// Package represents a NPM package
type Package struct {
	Name     string
	Version  string
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

// PackageMetadataVersion https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md#version
type PackageMetadataVersion struct {
	ID              string              `json:"_id"`
	Name            string              `json:"name"`
	Version         string              `json:"version"`
	Description     string              `json:"description"`
	Author          User                `json:"author"`
	Homepage        string              `json:"homepage,omitempty"`
	License         string              `json:"license,omitempty"`
	Repository      Repository          `json:"repository,omitempty"`
	Keywords        []string            `json:"keywords,omitempty"`
	Dependencies    map[string]string   `json:"dependencies,omitempty"`
	DevDependencies map[string]string   `json:"devDependencies,omitempty"`
	Readme          string              `json:"readme,omitempty"`
	Dist            PackageDistribution `json:"dist"`
	Maintainers     []User              `json:"maintainers,omitempty"`
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

// User https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md#package
type User struct {
	Username string `json:"username,omitempty"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	URL      string `json:"url,omitempty"`
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

// ParsePackage parses the content into a NPM package
func ParsePackage(r io.Reader) (*Package, error) {
	var upload packageUpload
	if err := json.NewDecoder(r).Decode(&upload); err != nil {
		return nil, err
	}

	for key, meta := range upload.Versions {
		if !validateName(meta.Name) {
			return nil, ErrInvalidPackageName
		}

		_, err := version.NewSemver(key)
		if err != nil {
			return nil, ErrInvalidPackageVersion
		}

		nameParts := strings.SplitN(meta.Name, "/", 2)

		if !validation.IsValidURL(meta.Homepage) {
			meta.Homepage = ""
		}

		p := &Package{
			Name:    meta.Name,
			Version: meta.Version,
			Metadata: Metadata{
				Scope:        nameParts[0],
				Name:         nameParts[1],
				Description:  meta.Description,
				Author:       meta.Author.Name,
				License:      meta.License,
				ProjectURL:   meta.Homepage,
				Dependencies: meta.Dependencies,
				Readme:       meta.Readme,
			},
		}

		names := strings.SplitN(p.Name, "/", 2)
		name := names[0]
		if len(names) == 2 {
			name = names[1]
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
