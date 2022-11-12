// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package container

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/packages/container/helm"
	"code.gitea.io/gitea/modules/packages/container/oci"

	"github.com/stretchr/testify/assert"
)

func TestParseImageConfig(t *testing.T) {
	description := "Image Description"
	author := "Gitea"
	license := "MIT"
	projectURL := "https://gitea.io"
	repositoryURL := "https://gitea.com/gitea"
	documentationURL := "https://docs.gitea.io"

	configOCI := `{"config": {"labels": {"` + labelAuthors + `": "` + author + `", "` + labelLicenses + `": "` + license + `", "` + labelURL + `": "` + projectURL + `", "` + labelSource + `": "` + repositoryURL + `", "` + labelDocumentation + `": "` + documentationURL + `", "` + labelDescription + `": "` + description + `"}}, "history": [{"created_by": "do it 1"}, {"created_by": "dummy #(nop) do it 2"}]}`

	metadata, err := ParseImageConfig(oci.MediaType(oci.MediaTypeImageManifest), strings.NewReader(configOCI))
	assert.NoError(t, err)

	assert.Equal(t, TypeOCI, metadata.Type)
	assert.Equal(t, description, metadata.Description)
	assert.ElementsMatch(t, []string{author}, metadata.Authors)
	assert.Equal(t, license, metadata.Licenses)
	assert.Equal(t, projectURL, metadata.ProjectURL)
	assert.Equal(t, repositoryURL, metadata.RepositoryURL)
	assert.Equal(t, documentationURL, metadata.DocumentationURL)
	assert.Equal(t, []string{"do it 1", "do it 2"}, metadata.ImageLayers)
	assert.Equal(
		t,
		map[string]string{
			labelAuthors:       author,
			labelLicenses:      license,
			labelURL:           projectURL,
			labelSource:        repositoryURL,
			labelDocumentation: documentationURL,
			labelDescription:   description,
		},
		metadata.Labels,
	)
	assert.Empty(t, metadata.MultiArch)

	configHelm := `{"description":"` + description + `", "home": "` + projectURL + `", "sources": ["` + repositoryURL + `"], "maintainers":[{"name":"` + author + `"}]}`

	metadata, err = ParseImageConfig(oci.MediaType(helm.ConfigMediaType), strings.NewReader(configHelm))
	assert.NoError(t, err)

	assert.Equal(t, TypeHelm, metadata.Type)
	assert.Equal(t, description, metadata.Description)
	assert.ElementsMatch(t, []string{author}, metadata.Authors)
	assert.Equal(t, projectURL, metadata.ProjectURL)
	assert.Equal(t, repositoryURL, metadata.RepositoryURL)
}
