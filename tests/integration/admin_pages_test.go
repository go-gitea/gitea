// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	pages_model "gitea.dev/models/pages"
	pages_service "gitea.dev/services/pages"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminPagesInstances(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	const adminPagesURL = "/-/admin/pages"

	resp := session.MakeRequest(t, NewRequest(t, "GET", adminPagesURL), http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	assert.Contains(t, doc.Find("h4.ui.header").Text(), "Pages instances (Total: 0)")
	assert.Equal(t, "No Pages instances have been created.", strings.TrimSpace(doc.Find("table tbody").Text()))
	createButton := doc.Find(`input[name="action"][value="create"]`).Closest("form").Find(`button[type="submit"]`)
	assert.Equal(t, "Create Pages instance?", createButton.AttrOr("data-modal-confirm-header", ""))
	assert.Equal(t, "A new instance credential will be generated and shown once.", createButton.AttrOr("data-modal-confirm-content", ""))

	session.MakeRequest(t, NewRequestWithValues(t, "POST", adminPagesURL, map[string]string{"action": "create"}), http.StatusSeeOther)
	resp = session.MakeRequest(t, NewRequest(t, "GET", adminPagesURL), http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)
	credentialInput := doc.Find(`input[value^="GITEA_PAGES_TOKEN="]`)
	require.Equal(t, 1, credentialInput.Length())
	credential, exists := credentialInput.Attr("value")
	require.True(t, exists)
	require.True(t, strings.HasPrefix(credential, "GITEA_PAGES_TOKEN="))
	assert.Len(t, strings.TrimPrefix(credential, "GITEA_PAGES_TOKEN="), 43)
	assert.Equal(t, credential, doc.Find(`[data-clipboard-text^="GITEA_PAGES_TOKEN="]`).AttrOr("data-clipboard-text", ""))
	assert.Contains(t, doc.Find("table tbody").Text(), "Waiting for the Pages service to connect")
	assert.Contains(t, doc.Find("table tbody").Text(), "Never")
	assert.Contains(t, doc.Find("h4.ui.header").Text(), "Pages instances (Total: 1)")
	assert.Equal(t, 4, doc.Find("table thead th").Length())
	assert.Equal(t, "Op.", strings.TrimSpace(doc.Find("table thead th:last-child").Text()))
	assert.Equal(t, 1, doc.Find(`table tbody a[data-url*="action=reset"]`).Length())
	assert.Equal(t, 1, doc.Find(`table tbody a[data-url*="action=remove"]`).Length())

	resp = session.MakeRequest(t, NewRequest(t, "GET", adminPagesURL), http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)
	assert.Equal(t, 0, doc.Find(`input[value^="GITEA_PAGES_TOKEN="]`).Length())
	assert.NotContains(t, resp.Body.String(), credential)

	instances, err := pages_model.ListInstances(t.Context())
	require.NoError(t, err)
	require.Len(t, instances, 1)
	_, err = pages_service.RegisterInstanceRootURL(t.Context(), instances[0].ID, instances[0].TokenHash, "http://pages-instance.example.test")
	require.NoError(t, err)
	require.NoError(t, pages_service.RecordInstanceCommunication(t.Context(), instances[0]))

	resp = session.MakeRequest(t, NewRequest(t, "GET", adminPagesURL), http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)
	assert.Equal(t, "http://pages-instance.example.test", strings.TrimSpace(doc.Find(`table tbody a[href="http://pages-instance.example.test"]`).Text()))
	assert.Equal(t, strconv.FormatInt(instances[0].ID, 10), strings.TrimSpace(doc.Find("table tbody td:first-child").Text()))
	assert.Equal(t, 1, doc.Find("table tbody td:nth-child(3) relative-time").Length())
	assert.NotContains(t, doc.Find("table tbody").Text(), "Waiting for the Pages service to connect")

	session.MakeRequest(t, NewRequestWithValues(t, "POST", adminPagesURL, map[string]string{
		"action":      "reset",
		"instance_id": strconv.FormatInt(instances[0].ID, 10),
	}), http.StatusSeeOther)
	resp = session.MakeRequest(t, NewRequest(t, "GET", adminPagesURL), http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)
	resetCredential := doc.Find(`input[value^="GITEA_PAGES_TOKEN="]`).AttrOr("value", "")
	assert.NotEmpty(t, resetCredential)
	assert.NotEqual(t, credential, resetCredential)
	assert.Equal(t, "Never", strings.TrimSpace(doc.Find("table tbody td:nth-child(3)").Text()))

	resp = session.MakeRequest(t, NewRequest(t, "GET", adminPagesURL), http.StatusOK)
	assert.NotContains(t, resp.Body.String(), resetCredential)

	session.MakeRequest(t, NewRequestWithValues(t, "POST", adminPagesURL, map[string]string{
		"action":      "remove",
		"instance_id": strconv.FormatInt(instances[0].ID, 10),
	}), http.StatusSeeOther)
	instances, err = pages_model.ListInstances(t.Context())
	require.NoError(t, err)
	assert.Empty(t, instances)
}
