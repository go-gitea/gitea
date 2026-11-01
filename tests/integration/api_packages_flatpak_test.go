// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"testing"

	packages_model "gitea.dev/models/packages"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/tests"

	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageFlatpak(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	req := NewRequest(t, "GET", setting.AppURL+"v2/token").AddBasicAuth(user.Name)
	tokenResponse := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &struct {
		Token string `json:"token"`
	}{})
	token := "Bearer " + tokenResponse.Token

	digestOf := func(content []byte) string {
		sum := sha256.Sum256(content)
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	uploadBlob := func(t *testing.T, image string, content []byte) string {
		digest := digestOf(content)
		req := NewRequestWithBody(t, "POST", fmt.Sprintf("%sv2/%s/%s/blobs/uploads?digest=%s", setting.AppURL, user.Name, image, digest), bytes.NewReader(content)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		return digest
	}
	putManifest := func(t *testing.T, image, reference, mediaType string, manifest []byte) string {
		req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%sv2/%s/%s/manifests/%s", setting.AppURL, user.Name, image, reference), bytes.NewReader(manifest)).
			AddTokenAuth(token).
			SetHeader("Content-Type", mediaType)
		MakeRequest(t, req, http.StatusCreated)
		return digestOf(manifest)
	}
	pushImage := func(t *testing.T, image, reference, arch string, labels map[string]string) (string, int) {
		layer := []byte("layer " + image + " " + arch)
		layerDigest := uploadBlob(t, image, layer)
		config, err := json.Marshal(map[string]any{
			"architecture": arch,
			"os":           "linux",
			"config":       map[string]any{"Labels": labels},
			"rootfs":       map[string]any{"type": "layers", "diff_ids": []string{layerDigest}},
		})
		require.NoError(t, err)
		configDigest := uploadBlob(t, image, config)
		manifest := fmt.Appendf(nil, `{"schemaVersion":2,"mediaType":"%s","config":{"mediaType":"%s","digest":"%s","size":%d},"layers":[{"mediaType":"%s","digest":"%s","size":%d}]}`,
			oci.MediaTypeImageManifest, oci.MediaTypeImageConfig, configDigest, len(config), oci.MediaTypeImageLayer, layerDigest, len(layer))
		if reference == "" {
			reference = digestOf(manifest)
		}
		return putManifest(t, image, reference, oci.MediaTypeImageManifest, manifest), len(manifest)
	}

	type flatpakIndex struct {
		Registry string
		Results  []struct {
			Name   string
			Images []struct {
				Tags         []string
				Digest       string
				MediaType    string
				OS           string
				Architecture string
				Labels       map[string]string
			}
		}
	}
	getIndex := func(t *testing.T, query string) *flatpakIndex {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/packages/%s/flatpak/index/static?label:org.flatpak.ref:exists=1&os=linux&%s", user.Name, query))
		return DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &flatpakIndex{})
	}
	imageType := func(t *testing.T, image string) packages_model.Type {
		for _, pt := range []packages_model.Type{packages_model.TypeFlatpak, packages_model.TypeContainer} {
			exists, err := packages_model.ExistVersion(t.Context(), &packages_model.PackageSearchOptions{
				OwnerID:    user.ID,
				Type:       pt,
				Name:       packages_model.SearchValue{ExactMatch: true, Value: image},
				IsInternal: optional.Some(false),
			})
			require.NoError(t, err)
			if exists {
				return pt
			}
		}
		return ""
	}

	flatpakLabels := func(arch string) map[string]string {
		return map[string]string{
			"org.flatpak.ref":      "app/org.example.App/" + arch + "/stable",
			"org.flatpak.metadata": "[Application]\nname=org.example.App\n",
		}
	}

	t.Run("Flatpak", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		digest, _ := pushImage(t, "org.example.app", "latest", "amd64", flatpakLabels("x86_64"))
		assert.Equal(t, packages_model.TypeFlatpak, imageType(t, "org.example.app"))

		index := getIndex(t, "architecture=amd64&tag=latest")
		assert.Equal(t, setting.AppURL, index.Registry)
		require.Len(t, index.Results, 1)
		assert.Equal(t, user.LowerName+"/org.example.app", index.Results[0].Name)
		require.Len(t, index.Results[0].Images, 1)
		img := index.Results[0].Images[0]
		assert.Equal(t, digest, img.Digest)
		assert.Equal(t, oci.MediaTypeImageManifest, img.MediaType)
		assert.Equal(t, []string{"latest"}, img.Tags)
		assert.Equal(t, "linux", img.OS)
		assert.Equal(t, "amd64", img.Architecture)
		assert.Equal(t, "app/org.example.App/x86_64/stable", img.Labels["org.flatpak.ref"])

		assert.Empty(t, getIndex(t, "architecture=arm64&tag=latest").Results)
		assert.Empty(t, getIndex(t, "architecture=amd64&tag=other").Results)

		// pulled like any other image
		req := NewRequest(t, "GET", fmt.Sprintf("%sv2/%s/org.example.app/manifests/latest", setting.AppURL, user.Name)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, digest, resp.Header().Get("Docker-Content-Digest"))

		req = NewRequest(t, "GET", fmt.Sprintf("%sv2/%s/org.example.app/tags/list", setting.AppURL, user.Name)).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), `"latest"`)
	})

	t.Run("Container", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		pushImage(t, "plain", "latest", "amd64", nil)
		assert.Equal(t, packages_model.TypeContainer, imageType(t, "plain"))

		// an existing container image is not turned into a Flatpak
		pushImage(t, "plain", "labelled", "amd64", flatpakLabels("x86_64"))
		assert.Equal(t, packages_model.TypeContainer, imageType(t, "plain"))

		for _, result := range getIndex(t, "").Results {
			assert.NotEqual(t, user.LowerName+"/plain", result.Name)
		}
	})

	t.Run("Index", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		image := "org.example.multi"
		amd64Digest, amd64Size := pushImage(t, image, "", "amd64", flatpakLabels("x86_64"))
		arm64Digest, arm64Size := pushImage(t, image, "", "arm64", flatpakLabels("aarch64"))
		index := fmt.Appendf(nil, `{"schemaVersion":2,"mediaType":"%s","manifests":[{"mediaType":"%s","digest":"%s","size":%d,"platform":{"os":"linux","architecture":"amd64"}},{"mediaType":"%s","digest":"%s","size":%d,"platform":{"os":"linux","architecture":"arm64"}}]}`,
			oci.MediaTypeImageIndex, oci.MediaTypeImageManifest, amd64Digest, amd64Size, oci.MediaTypeImageManifest, arm64Digest, arm64Size)
		putManifest(t, image, "stable", oci.MediaTypeImageIndex, index)

		// the index has no labels of its own and joins the Flatpak
		assert.Equal(t, packages_model.TypeFlatpak, imageType(t, image))

		result := getIndex(t, "architecture=arm64&tag=stable")
		require.Len(t, result.Results, 1)
		require.Len(t, result.Results[0].Images, 1)
		assert.Equal(t, arm64Digest, result.Results[0].Images[0].Digest)
		assert.Equal(t, []string{"stable"}, result.Results[0].Images[0].Tags)
		assert.True(t, strings.HasPrefix(result.Results[0].Images[0].Labels["org.flatpak.ref"], "app/org.example.App/aarch64/"))
	})
}
