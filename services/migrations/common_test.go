// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseServiceCloneURL(t *testing.T) {
	tests := []struct {
		name        string
		cloneURL    string
		apiURL      string
		repoPath    string
		pathSegment []string
	}{
		{
			name:        "https",
			cloneURL:    "https://github.com/go-gitea/gitea.git",
			apiURL:      "https://github.com",
			repoPath:    "go-gitea/gitea.git",
			pathSegment: []string{"go-gitea", "gitea.git"},
		},
		{
			name:        "ssh",
			cloneURL:    "ssh://github.com/go-gitea/gitea.git",
			apiURL:      "https://github.com",
			repoPath:    "go-gitea/gitea.git",
			pathSegment: []string{"go-gitea", "gitea.git"},
		},
		{
			name:        "git protocol",
			cloneURL:    "git://github.com/go-gitea/gitea.git",
			apiURL:      "https://github.com",
			repoPath:    "go-gitea/gitea.git",
			pathSegment: []string{"go-gitea", "gitea.git"},
		},
		{
			name:        "sub-path instance",
			cloneURL:    "ssh://example.com/gitbucket/team/repo.git",
			apiURL:      "https://example.com",
			repoPath:    "gitbucket/team/repo.git",
			pathSegment: []string{"gitbucket", "team", "repo.git"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseServiceCloneURL(tt.cloneURL)
			require.NoError(t, err)
			assert.Equal(t, tt.apiURL, info.apiURL.String())
			assert.Equal(t, tt.repoPath, info.repoPath)
			assert.Equal(t, tt.pathSegment, info.segments)
		})
	}
}

func TestGithubDownloaderFactoryNewWithSSHCloneURL(t *testing.T) {
	downloader, err := (&GithubDownloaderV3Factory{}).New(t.Context(), MigrateOptions{
		CloneAddr: "ssh://github.com/go-gitea/gitea.git",
	})
	require.NoError(t, err)

	githubDownloader, ok := downloader.(*GithubDownloaderV3)
	require.True(t, ok)
	assert.Equal(t, "https://github.com", githubDownloader.baseURL)
	assert.Equal(t, "go-gitea", githubDownloader.repoOwner)
	assert.Equal(t, "gitea", githubDownloader.repoName)
}

func TestGitBucketDownloaderFactoryNewWithSSHCloneURL(t *testing.T) {
	downloader, err := (&GitBucketDownloaderFactory{}).New(t.Context(), MigrateOptions{
		CloneAddr: "ssh://example.com/git/team/repo.git",
	})
	require.NoError(t, err)

	gitBucketDownloader, ok := downloader.(*GitBucketDownloader)
	require.True(t, ok)
	assert.Equal(t, "https://example.com", gitBucketDownloader.baseURL)
	assert.Equal(t, "team", gitBucketDownloader.repoOwner)
	assert.Equal(t, "repo", gitBucketDownloader.repoName)
}

func TestCodeCommitDownloaderFactoryNewWithSSHCloneURL(t *testing.T) {
	downloader, err := (&CodeCommitDownloaderFactory{}).New(t.Context(), MigrateOptions{
		CloneAddr: "ssh://git-codecommit.us-east-1.amazonaws.com/v1/repos/test-repo",
	})
	require.NoError(t, err)

	codeCommitDownloader, ok := downloader.(*CodeCommitDownloader)
	require.True(t, ok)
	assert.Equal(t, "https://git-codecommit.us-east-1.amazonaws.com", codeCommitDownloader.baseURL)
	assert.Equal(t, "test-repo", codeCommitDownloader.repoName)
}
