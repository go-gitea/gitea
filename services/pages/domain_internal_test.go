// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/require"
)

func TestIsTXTRecordAbsent(t *testing.T) {
	require.True(t, isTXTRecordAbsent(&net.DNSError{IsNotFound: true}))
	require.True(t, isTXTRecordAbsent(fmt.Errorf("wrapped: %w", &net.DNSError{IsNotFound: true})))
	require.False(t, isTXTRecordAbsent(&net.DNSError{IsTimeout: true, IsTemporary: true}))
	require.False(t, isTXTRecordAbsent(errors.New("resolver unavailable")))
}

func TestDomainPublishingGrace(t *testing.T) {
	now := time.Now().UnixNano()
	domain := &pages_model.Domain{
		State: pages_model.DomainStateActive, TXTResult: pages_model.TXTResultInvalid,
		TXTLastSuccessUnixNano: now - int64(time.Hour),
	}
	require.True(t, domainPublishingEligible(domain, true, now))
	domain.TXTLastSuccessUnixNano = now - int64(25*time.Hour)
	require.False(t, domainPublishingEligible(domain, true, now))
	domain.TXTLastSuccessUnixNano = now + int64(time.Hour)
	require.False(t, domainPublishingEligible(domain, true, now))
	domain.State = pages_model.DomainStatePending
	require.False(t, domainPublishingEligible(domain, true, now))
}

func TestListDomainsDueForVerificationUsesConfiguredInterval(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	previousRequirement := setting.Pages.RequireGiteaTXTRecord
	previousInterval := setting.Pages.DomainVerificationInterval
	setting.Pages.RequireGiteaTXTRecord = true
	setting.Pages.DomainVerificationInterval = 15 * time.Minute
	t.Cleanup(func() {
		setting.Pages.RequireGiteaTXTRecord = previousRequirement
		setting.Pages.DomainVerificationInterval = previousInterval
	})
	now := int64(timeutil.TimeStampNow())
	domains := []*pages_model.Domain{
		{RepoID: 1, Host: "first.example.test", TXTToken: "first", State: pages_model.DomainStatePending, TXTResult: pages_model.TXTResultPending, CreatedUnixNano: time.Now().UnixNano()},
		{RepoID: 2, Host: "due.example.test", TXTToken: "due", State: pages_model.DomainStateActive, TXTResult: pages_model.TXTResultValid, TXTLastCheckUnix: now - int64(20*time.Minute/time.Second), CreatedUnixNano: time.Now().UnixNano()},
		{RepoID: 3, Host: "fresh.example.test", TXTToken: "fresh", State: pages_model.DomainStateActive, TXTResult: pages_model.TXTResultValid, TXTLastCheckUnix: now - int64(10*time.Minute/time.Second), CreatedUnixNano: time.Now().UnixNano()},
	}
	for _, domain := range domains {
		require.NoError(t, pages_model.InsertDomain(t.Context(), domain))
	}

	due, err := listDomainsDueForVerification(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, due, 2)
	require.Equal(t, domains[0].ID, due[0].ID)
	require.Equal(t, domains[1].ID, due[1].ID)
}

func TestPagesTXTResolverUsesConfiguredAddress(t *testing.T) {
	previousResolver := setting.Pages.DNSResolver
	setting.Pages.DNSResolver = "127.0.0.1:5353"
	t.Cleanup(func() { setting.Pages.DNSResolver = previousResolver })

	resolver := pagesTXTResolver()
	require.NotNil(t, resolver.Dial)
	connection, err := resolver.Dial(t.Context(), "udp", "127.0.0.1:53")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	require.Equal(t, setting.Pages.DNSResolver, connection.RemoteAddr().String())
}

func TestVerifyDomainKeepsActiveBindingDuringGrace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	previousRequirement := setting.Pages.RequireGiteaTXTRecord
	setting.Pages.RequireGiteaTXTRecord = true
	t.Cleanup(func() { setting.Pages.RequireGiteaTXTRecord = previousRequirement })
	_, err := SaveSiteSettings(t.Context(), 1, SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	domain, err := AddDomain(t.Context(), 1, "grace.example.test")
	require.NoError(t, err)
	domain.TXTToken = "token"
	require.NoError(t, pages_model.UpdateDomain(t.Context(), domain))
	require.NoError(t, verifyDomain(t.Context(), domain.ID, func(_ context.Context, _ string) ([]string, error) {
		return []string{"token"}, nil
	}))
	domain, _, err = pages_model.GetDomainByID(t.Context(), domain.ID)
	require.NoError(t, err)
	lastSuccess := domain.TXTLastSuccessUnixNano
	site, _, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	previousUpdate := site.PagesUpdatedUnixNano

	require.NoError(t, verifyDomain(t.Context(), domain.ID, func(_ context.Context, _ string) ([]string, error) {
		return []string{"different"}, nil
	}))
	domain, _, err = pages_model.GetDomainByID(t.Context(), domain.ID)
	require.NoError(t, err)
	require.Equal(t, pages_model.TXTResultInvalid, domain.TXTResult)
	require.Equal(t, lastSuccess, domain.TXTLastSuccessUnixNano)
	require.True(t, domainPublishingEligible(domain, true, time.Now().UnixNano()))
	site, _, err = pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.Greater(t, site.PagesUpdatedUnixNano, previousUpdate)
}

func TestVerifyDomainRecordsEveryTXTAttempt(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		lookup     func(context.Context, string) ([]string, error)
		result     pages_model.TXTResult
		state      pages_model.DomainState
		hasSuccess bool
	}{
		{
			name: "valid",
			lookup: func(_ context.Context, _ string) ([]string, error) {
				return []string{"token"}, nil
			},
			result: pages_model.TXTResultValid, state: pages_model.DomainStateActive, hasSuccess: true,
		},
		{
			name: "invalid",
			lookup: func(_ context.Context, _ string) ([]string, error) {
				return []string{"different"}, nil
			},
			result: pages_model.TXTResultInvalid, state: pages_model.DomainStatePending,
		},
		{
			name: "temporary error",
			lookup: func(_ context.Context, _ string) ([]string, error) {
				return nil, errors.New("resolver unavailable")
			},
			result: pages_model.TXTResultTemporaryError, state: pages_model.DomainStatePending,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			previousRequirement := setting.Pages.RequireGiteaTXTRecord
			setting.Pages.RequireGiteaTXTRecord = true
			t.Cleanup(func() { setting.Pages.RequireGiteaTXTRecord = previousRequirement })
			_, err := SaveSiteSettings(t.Context(), 1, SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
			require.NoError(t, err)
			domain, err := AddDomain(t.Context(), 1, "check.example.test")
			require.NoError(t, err)
			domain.TXTToken = "token"
			require.NoError(t, pages_model.UpdateDomain(t.Context(), domain))

			require.NoError(t, verifyDomain(t.Context(), domain.ID, testCase.lookup))
			domain, has, err := pages_model.GetDomainByID(t.Context(), domain.ID)
			require.NoError(t, err)
			require.True(t, has)
			require.Equal(t, testCase.result, domain.TXTResult)
			require.Equal(t, testCase.state, domain.State)
			require.Positive(t, domain.TXTLastCheckUnix)
			require.Equal(t, testCase.hasSuccess, domain.TXTLastSuccessUnixNano > 0)
		})
	}
}
