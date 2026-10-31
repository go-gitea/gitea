// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	test_module "gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	pages_service "gitea.dev/services/pages"

	"github.com/stretchr/testify/require"
)

func TestPagesInstanceRoutesUseTheirOwnCredential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test_module.MockVariableValue(&setting.InternalToken, "gitea-process-token")()
	instance, token, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	router := Routes()
	firstCommunication := time.Date(2026, time.October, 30, 10, 0, 0, 0, time.UTC)
	timeutil.MockSet(firstCommunication)
	defer timeutil.MockUnset()

	register := httptest.NewRequest(http.MethodPut, "/pages/instance", strings.NewReader(`{"root_url":"http://pages.example.test"}`)).WithContext(reqctx.NewRequestContextForTest(t))
	register.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, register)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	stored, has, err := pages_model.GetInstanceByID(t.Context(), instance.ID)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, timeutil.TimeStamp(firstCommunication.Unix()), stored.LastCommunicationUnix)

	timeutil.MockSet(firstCommunication.Add(30 * time.Second))
	list := httptest.NewRequest(http.MethodGet, "/pages/sites?limit=10", nil).WithContext(reqctx.NewRequestContextForTest(t))
	list.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, list)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), `"root_url":"http://pages.example.test"`)
	stored, has, err = pages_model.GetInstanceByID(t.Context(), instance.ID)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, timeutil.TimeStamp(firstCommunication.Unix()), stored.LastCommunicationUnix)

	timeutil.MockSet(firstCommunication.Add(time.Minute))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, list.Clone(reqctx.NewRequestContextForTest(t)))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	stored, has, err = pages_model.GetInstanceByID(t.Context(), instance.ID)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, timeutil.TimeStamp(firstCommunication.Add(time.Minute).Unix()), stored.LastCommunicationUnix)

	timeutil.MockSet(firstCommunication.Add(2 * time.Minute))
	invalid := httptest.NewRequest(http.MethodGet, "/pages/sites?limit=10", nil).WithContext(reqctx.NewRequestContextForTest(t))
	invalid.Header.Set("Authorization", "Bearer invalid")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, invalid)
	require.Equal(t, http.StatusForbidden, response.Code)
	stored, has, err = pages_model.GetInstanceByID(t.Context(), instance.ID)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, timeutil.TimeStamp(firstCommunication.Add(time.Minute).Unix()), stored.LastCommunicationUnix)
}
