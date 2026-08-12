// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"slices"
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// localStorageCheck expects a storage to be local and rooted at a given absolute path.
type localStorageCheck struct {
	name     string
	ptr      **Storage
	wantPath string
}

func (c *localStorageCheck) snapshot() func() { return test.MockVariableValue(c.ptr) }

func (c *localStorageCheck) check(t *testing.T) {
	t.Helper()
	s := *c.ptr
	assert.Equal(t, LocalStorageType, s.Type, "storage %s type", c.name)
	assert.True(t, filepath.IsAbs(s.Path), "storage %s path %q is not absolute", c.name, s.Path)
	assert.Equal(t, filepath.Clean(c.wantPath), filepath.Clean(s.Path), "storage %s path", c.name)
}

func localStorage(name string, ptr **Storage, wantPath string) configCheck {
	return &localStorageCheck{name: name, ptr: ptr, wantPath: wantPath}
}

// minioStorageAt expects a minio storage at a base path, guarding the pointer the loader replaces.
func minioStorageAt(name string, ptr **Storage, basePath string) []configCheck {
	return []configCheck{
		guard(ptr),
		fieldOf(name+" STORAGE_TYPE", func() StorageType { return (*ptr).Type }, MinioStorageType),
		fieldOf(name+" MINIO_BASE_PATH", func() string { return (*ptr).MinioConfig.BasePath }, basePath),
	}
}

// minioStorage is minioStorageAt with the bucket asserted too.
func minioStorage(name string, ptr **Storage, bucket, basePath string) []configCheck {
	return slices.Concat(minioStorageAt(name, ptr, basePath), []configCheck{
		fieldOf(name+" MINIO_BUCKET", func() string { return (*ptr).MinioConfig.Bucket }, bucket),
	})
}

// azureBlobStorage expects an azureblob storage, guarding the pointer the loader replaces.
func azureBlobStorage(name string, ptr **Storage, container, basePath string) []configCheck {
	return []configCheck{
		guard(ptr),
		fieldOf(name+" STORAGE_TYPE", func() StorageType { return (*ptr).Type }, AzureBlobStorageType),
		fieldOf(name+" AZURE_BLOB_CONTAINER", func() string { return (*ptr).AzureBlobConfig.Container }, container),
		fieldOf(name+" AZURE_BLOB_BASE_PATH", func() string { return (*ptr).AzureBlobConfig.BasePath }, basePath),
	}
}

var allStorageLoaders = []any{
	loadAttachmentFrom, loadLFSFrom, loadActionsFrom, loadPackagesFrom,
	loadRepoArchiveFrom, loadAvatarsFrom, loadRepoAvatarFrom,
}

func Test_getStorageMultipleName(t *testing.T) {
	defer test.MockVariableValue(&Attachment)()
	defer test.MockVariableValue(&LFS)()
	defer test.MockVariableValue(&Avatar)()

	testConfigLoad(t, []any{loadAttachmentFrom, loadLFSFrom, loadAvatarsFrom}, []configTestCase{
		{
			name: "each section keeps its own bucket, the rest inherit [storage]",
			ini: `
[lfs]
MINIO_BUCKET = gitea-lfs

[attachment]
MINIO_BUCKET = gitea-attachment

[storage]
STORAGE_TYPE = minio
MINIO_BUCKET = gitea-storage
`,
			want: slices.Concat(
				minioStorage("attachment", &Attachment.Storage, "gitea-attachment", "attachments/"),
				minioStorage("lfs", &LFS.Storage, "gitea-lfs", "lfs/"),
				minioStorage("avatar", &Avatar.Storage, "gitea-storage", "avatars/"),
			),
		},
	})
}

func Test_getStorageUseOtherNameAsType(t *testing.T) {
	defer test.MockVariableValue(&Attachment)()
	defer test.MockVariableValue(&LFS)()

	testConfigLoad(t, []any{loadAttachmentFrom, loadLFSFrom}, []configTestCase{
		{
			name: "attachment borrows the lfs storage but keeps its own base path",
			ini: `
[attachment]
STORAGE_TYPE = lfs

[storage.lfs]
STORAGE_TYPE = minio
MINIO_BUCKET = gitea-storage
`,
			want: slices.Concat(
				minioStorage("attachment", &Attachment.Storage, "gitea-storage", "attachments/"),
				minioStorage("lfs", &LFS.Storage, "gitea-storage", "lfs/"),
			),
		},
	})
}

func Test_getStorageInheritStorageType(t *testing.T) {
	defer test.MockVariableValue(&Packages)()
	defer test.MockVariableValue(&RepoArchive)()
	defer test.MockVariableValue(&Actions)()
	defer test.MockVariableValue(&Avatar)()
	defer test.MockVariableValue(&RepoAvatar)()

	testConfigLoad(t, []any{loadPackagesFrom, loadRepoArchiveFrom, loadActionsFrom, loadAvatarsFrom, loadRepoAvatarFrom}, []configTestCase{
		{
			name: "minio",
			ini:  "[storage]\nSTORAGE_TYPE = minio",
			want: slices.Concat(
				minioStorage("packages", &Packages.Storage, "gitea", "packages/"),
				minioStorage("repo-archive", &RepoArchive.Storage, "gitea", "repo-archive/"),
				minioStorage("actions_log", &Actions.LogStorage, "gitea", "actions_log/"),
				minioStorage("actions_artifacts", &Actions.ArtifactStorage, "gitea", "actions_artifacts/"),
				minioStorage("avatar", &Avatar.Storage, "gitea", "avatars/"),
				minioStorage("repo-avatar", &RepoAvatar.Storage, "gitea", "repo-avatars/"),
			),
		},
		{
			name: "azureblob",
			ini:  "[storage]\nSTORAGE_TYPE = azureblob",
			want: slices.Concat(
				azureBlobStorage("packages", &Packages.Storage, "gitea", "packages/"),
				azureBlobStorage("repo-archive", &RepoArchive.Storage, "gitea", "repo-archive/"),
				azureBlobStorage("actions_log", &Actions.LogStorage, "gitea", "actions_log/"),
				azureBlobStorage("actions_artifacts", &Actions.ArtifactStorage, "gitea", "actions_artifacts/"),
				azureBlobStorage("avatar", &Avatar.Storage, "gitea", "avatars/"),
				azureBlobStorage("repo-avatar", &RepoAvatar.Storage, "gitea", "repo-avatars/"),
			),
		},
	})
}

func Test_getStorageInheritStorageTypeLocal(t *testing.T) {
	defer test.MockVariableValue(&AppDataPath, "/appdata")()
	defer test.MockVariableValue(&Attachment)()
	defer test.MockVariableValue(&LFS)()
	defer test.MockVariableValue(&Actions)()
	defer test.MockVariableValue(&Packages)()
	defer test.MockVariableValue(&RepoArchive)()
	defer test.MockVariableValue(&Avatar)()
	defer test.MockVariableValue(&RepoAvatar)()

	// every case asserts all storages, archive overriding only the repo-archive one
	under := func(root, archive string) []configCheck {
		if archive == "" {
			archive = root + "/repo-archive"
		}
		return []configCheck{
			localStorage("attachment", &Attachment.Storage, root+"/attachments"),
			localStorage("lfs", &LFS.Storage, root+"/lfs"),
			localStorage("actions_artifacts", &Actions.ArtifactStorage, root+"/actions_artifacts"),
			localStorage("packages", &Packages.Storage, root+"/packages"),
			localStorage("repo-archive", &RepoArchive.Storage, archive),
			localStorage("actions_log", &Actions.LogStorage, root+"/actions_log"),
			localStorage("avatar", &Avatar.Storage, root+"/avatars"),
			localStorage("repo-avatar", &RepoAvatar.Storage, root+"/repo-avatars"),
		}
	}

	testConfigLoad(t, allStorageLoaders, []configTestCase{
		{
			name: "everything under APP_DATA_PATH",
			ini:  "[storage]\nSTORAGE_TYPE = local",
			want: under("/appdata", ""),
		},
		{
			name: "an absolute [storage].PATH becomes the root",
			ini:  "[storage]\nSTORAGE_TYPE = local\nPATH = /data/gitea",
			want: under("/data/gitea", ""),
		},
		{
			name: "a relative [storage].PATH is resolved against APP_DATA_PATH",
			ini:  "[storage]\nSTORAGE_TYPE = local\nPATH = storages",
			want: under("/appdata/storages", ""),
		},
		{
			name: "an absolute [repo-archive].PATH overrides only that storage",
			ini:  "[storage]\nSTORAGE_TYPE = local\nPATH = /data/gitea\n\n[repo-archive]\nPATH = /data/gitea/the-archives-dir",
			want: under("/data/gitea", "/data/gitea/the-archives-dir"),
		},
		{
			name: "an empty [repo-archive] changes nothing",
			ini:  "[storage]\nSTORAGE_TYPE = local\nPATH = /data/gitea\n\n[repo-archive]",
			want: under("/data/gitea", ""),
		},
		{
			name: "a relative [repo-archive].PATH is resolved against the root",
			ini:  "[storage]\nSTORAGE_TYPE = local\nPATH = /data/gitea\n\n[repo-archive]\nPATH = the-archives-dir",
			want: under("/data/gitea", "/data/gitea/the-archives-dir"),
		},
		{
			name: "an absolute [storage.repo-archive].PATH overrides only that storage",
			ini:  "[storage.repo-archive]\nSTORAGE_TYPE = local\nPATH = /data/gitea/archives",
			want: under("/appdata", "/data/gitea/archives"),
		},
		{
			name: "a relative [storage.repo-archive].PATH is resolved against APP_DATA_PATH",
			ini:  "[storage.repo-archive]\nSTORAGE_TYPE = local\nPATH = a-relative-path",
			want: under("/appdata", "/appdata/a-relative-path"),
		},
		{
			name: "[repo-archive].PATH wins over [storage.repo-archive].PATH",
			ini:  "[storage.repo-archive]\nSTORAGE_TYPE = local\nPATH = /data/gitea/archives\n\n[repo-archive]\nPATH = /tmp/gitea/archives",
			want: under("/appdata", "/tmp/gitea/archives"),
		},
		{
			name: "an empty [repo-archive] keeps [storage.repo-archive].PATH",
			ini:  "[storage.repo-archive]\nSTORAGE_TYPE = local\nPATH = /data/gitea/archives\n\n[repo-archive]",
			want: under("/appdata", "/data/gitea/archives"),
		},
		{
			name:    "a relative [repo-archive].PATH alone is resolved against APP_DATA_PATH",
			ini:     "[repo-archive]\nSTORAGE_TYPE = local\nPATH = archives",
			loaders: []any{loadRepoArchiveFrom},
			want:    []configCheck{localStorage("repo-archive", &RepoArchive.Storage, "/appdata/archives")},
		},
		{
			name:    "an empty [storage.repo-archive] falls back to the default dir",
			ini:     "[storage.repo-archive]",
			loaders: []any{loadRepoArchiveFrom},
			want:    []configCheck{localStorage("repo-archive", &RepoArchive.Storage, "/appdata/repo-archive")},
		},
		{
			name:    "a relative [storage.repo-archive].PATH without a type is still local",
			ini:     "[storage.repo-archive]\nPATH = archives",
			loaders: []any{loadRepoArchiveFrom},
			want:    []configCheck{localStorage("repo-archive", &RepoArchive.Storage, "/appdata/archives")},
		},
	})
}

func Test_getStorageUnnamedSectionIsRejected(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[repo-archive]
STORAGE_TYPE = minio
MINIO_ACCESS_KEY_ID = my_access_key
MINIO_SECRET_ACCESS_KEY = my_secret_key
`)
	require.NoError(t, err)

	_, err = getStorage(cfg, "", "", nil)
	assert.Error(t, err)
}

func Test_getStorageConfigurationErrors(t *testing.T) {
	defer test.MockVariableValue(&RepoArchive)()

	testConfigLoad(t, []any{loadRepoArchiveFrom}, []configTestCase{
		{
			name:    "an unknown named storage",
			ini:     "[repo-archive]\nSTORAGE_TYPE = my_storage\nPATH = archives",
			wantErr: assert.Error,
		},
		{
			name:    "a named storage without an explicit type",
			ini:     "[repo-archive]\nSTORAGE_TYPE = my_archive\n\n[storage.my_archive]\n; unsupported, storage type should be defined explicitly\nPATH = archives",
			wantErr: assert.Error,
		},
		{
			name:    "a named storage with an unknown type",
			ini:     "[repo-archive]\nSTORAGE_TYPE = my_archive\n\n[storage.my_archive]\n; unsupported, storage type should be known type\nSTORAGE_TYPE = unknown // should be local or minio\nPATH = archives",
			wantErr: assert.Error,
		},
		{
			name: "a malformed minio bool",
			ini:  "[repo-archive]\nSTORAGE_TYPE = minio\nMINIO_ACCESS_KEY_ID = my_access_key\nMINIO_SECRET_ACCESS_KEY = my_secret_key\n; wrong configuration\nMINIO_USE_SSL = abc",
			// FIXME: this should return error but now ini package's MapTo() doesn't check type
			wantErr: assert.NoError,
		},
		{
			name: "an azureblob storage in an unnamed section",
			ini:  "[repo-archive]\nSTORAGE_TYPE = azureblob\nAZURE_BLOB_ACCOUNT_NAME = my_account_name\nAZURE_BLOB_ACCOUNT_KEY = my_account_key",
			// FIXME: this should return error but now ini package's MapTo() doesn't check type
			wantErr: assert.NoError,
		},
	})
}

func Test_getStorageMinioConfiguration(t *testing.T) {
	defer test.MockVariableValue(&RepoArchive)()
	defer test.MockVariableValue(&LFS)()

	archiveMinio := func() MinioStorageConfig { return RepoArchive.Storage.MinioConfig }
	lfsMinio := func() MinioStorageConfig { return LFS.Storage.MinioConfig }

	testConfigLoad(t, []any{loadRepoArchiveFrom}, []configTestCase{
		{
			name: "credentials are shadowed in a copy",
			ini:  "[repo-archive]\nSTORAGE_TYPE = minio\nMINIO_ACCESS_KEY_ID = my_access_key\nMINIO_SECRET_ACCESS_KEY = my_secret_key",
			want: []configCheck{guard(&RepoArchive.Storage)},
			check: func(t *testing.T) {
				cp := RepoArchive.Storage.ToShadowCopy()
				assert.Equal(t, "******", cp.MinioConfig.AccessKeyID)
				assert.Equal(t, "******", cp.MinioConfig.SecretAccessKey)
			},
		},
		{
			name: "[storage.repo-archive] credentials",
			ini:  "[storage.repo-archive]\nSTORAGE_TYPE = minio\nMINIO_ACCESS_KEY_ID = my_access_key\nMINIO_SECRET_ACCESS_KEY = my_secret_key\nMINIO_USE_SSL = true",
			want: []configCheck{
				guard(&RepoArchive.Storage),
				fieldOf("MINIO_ACCESS_KEY_ID", func() string { return archiveMinio().AccessKeyID }, "my_access_key"),
				fieldOf("MINIO_SECRET_ACCESS_KEY", func() string { return archiveMinio().SecretAccessKey }, "my_secret_key"),
				fieldOf("MINIO_USE_SSL", func() bool { return archiveMinio().UseSSL }, true),
				fieldOf("MINIO_BASE_PATH", func() string { return archiveMinio().BasePath }, "repo-archive/"),
			},
		},
		{
			name: "[storage].MINIO_BASE_PATH is a prefix",
			ini:  "[storage]\nSTORAGE_TYPE = minio\nMINIO_ACCESS_KEY_ID = my_access_key\nMINIO_SECRET_ACCESS_KEY = my_secret_key\nMINIO_USE_SSL = true\nMINIO_BASE_PATH = /prefix",
			want: []configCheck{
				guard(&RepoArchive.Storage),
				fieldOf("MINIO_ACCESS_KEY_ID", func() string { return archiveMinio().AccessKeyID }, "my_access_key"),
				fieldOf("MINIO_SECRET_ACCESS_KEY", func() string { return archiveMinio().SecretAccessKey }, "my_secret_key"),
				fieldOf("MINIO_USE_SSL", func() bool { return archiveMinio().UseSSL }, true),
				fieldOf("MINIO_BASE_PATH", func() string { return archiveMinio().BasePath }, "/prefix/repo-archive/"),
			},
		},
		{
			name: "an IAM endpoint instead of credentials",
			ini:  "[storage]\nSTORAGE_TYPE = minio\nMINIO_IAM_ENDPOINT = 127.0.0.1\nMINIO_USE_SSL = true\nMINIO_BASE_PATH = /prefix",
			want: []configCheck{
				guard(&RepoArchive.Storage),
				fieldOf("MINIO_IAM_ENDPOINT", func() string { return archiveMinio().IamEndpoint }, "127.0.0.1"),
				fieldOf("MINIO_USE_SSL", func() bool { return archiveMinio().UseSSL }, true),
				fieldOf("MINIO_BASE_PATH", func() string { return archiveMinio().BasePath }, "/prefix/repo-archive/"),
			},
		},
		{
			name:    "[lfs].MINIO_BASE_PATH replaces the prefix",
			ini:     "[storage]\nSTORAGE_TYPE = minio\nMINIO_ACCESS_KEY_ID = my_access_key\nMINIO_SECRET_ACCESS_KEY = my_secret_key\nMINIO_USE_SSL = true\nMINIO_BASE_PATH = /prefix\n\n[lfs]\nMINIO_BASE_PATH = /lfs",
			loaders: []any{loadLFSFrom},
			want: []configCheck{
				guard(&LFS.Storage),
				fieldOf("MINIO_ACCESS_KEY_ID", func() string { return lfsMinio().AccessKeyID }, "my_access_key"),
				fieldOf("MINIO_SECRET_ACCESS_KEY", func() string { return lfsMinio().SecretAccessKey }, "my_secret_key"),
				fieldOf("MINIO_USE_SSL", func() bool { return lfsMinio().UseSSL }, true),
				fieldOf("MINIO_BASE_PATH", func() string { return lfsMinio().BasePath }, "/lfs"),
			},
		},
		{
			name:    "[storage.lfs].MINIO_BASE_PATH replaces the prefix",
			ini:     "[storage]\nSTORAGE_TYPE = minio\nMINIO_ACCESS_KEY_ID = my_access_key\nMINIO_SECRET_ACCESS_KEY = my_secret_key\nMINIO_USE_SSL = true\nMINIO_BASE_PATH = /prefix\n\n[storage.lfs]\nMINIO_BASE_PATH = /lfs",
			loaders: []any{loadLFSFrom},
			want: []configCheck{
				guard(&LFS.Storage),
				fieldOf("MINIO_ACCESS_KEY_ID", func() string { return lfsMinio().AccessKeyID }, "my_access_key"),
				fieldOf("MINIO_SECRET_ACCESS_KEY", func() string { return lfsMinio().SecretAccessKey }, "my_secret_key"),
				fieldOf("MINIO_USE_SSL", func() bool { return lfsMinio().UseSSL }, true),
				fieldOf("MINIO_BASE_PATH", func() string { return lfsMinio().BasePath }, "/lfs"),
			},
		},
	})
}

func Test_getStorageAzureBlobConfiguration(t *testing.T) {
	defer test.MockVariableValue(&RepoArchive)()
	defer test.MockVariableValue(&LFS)()

	archiveAzure := func() AzureBlobStorageConfig { return RepoArchive.Storage.AzureBlobConfig }
	lfsAzure := func() AzureBlobStorageConfig { return LFS.Storage.AzureBlobConfig }

	testConfigLoad(t, []any{loadRepoArchiveFrom}, []configTestCase{
		{
			name: "[storage.repo-archive] credentials",
			ini:  "[storage.repo-archive]\nSTORAGE_TYPE = azureblob\nAZURE_BLOB_ACCOUNT_NAME = my_account_name\nAZURE_BLOB_ACCOUNT_KEY = my_account_key",
			want: []configCheck{
				guard(&RepoArchive.Storage),
				fieldOf("AZURE_BLOB_ACCOUNT_NAME", func() string { return archiveAzure().AccountName }, "my_account_name"),
				fieldOf("AZURE_BLOB_ACCOUNT_KEY", func() string { return archiveAzure().AccountKey }, "my_account_key"),
				fieldOf("AZURE_BLOB_BASE_PATH", func() string { return archiveAzure().BasePath }, "repo-archive/"),
			},
		},
		{
			name: "[storage].AZURE_BLOB_BASE_PATH is a prefix",
			ini:  "[storage]\nSTORAGE_TYPE = azureblob\nAZURE_BLOB_ACCOUNT_NAME = my_account_name\nAZURE_BLOB_ACCOUNT_KEY = my_account_key\nAZURE_BLOB_BASE_PATH = /prefix",
			want: []configCheck{
				guard(&RepoArchive.Storage),
				fieldOf("AZURE_BLOB_ACCOUNT_NAME", func() string { return archiveAzure().AccountName }, "my_account_name"),
				fieldOf("AZURE_BLOB_ACCOUNT_KEY", func() string { return archiveAzure().AccountKey }, "my_account_key"),
				fieldOf("AZURE_BLOB_BASE_PATH", func() string { return archiveAzure().BasePath }, "/prefix/repo-archive/"),
			},
		},
		{
			name:    "[lfs].AZURE_BLOB_BASE_PATH replaces the prefix",
			ini:     "[storage]\nSTORAGE_TYPE = azureblob\nAZURE_BLOB_ACCOUNT_NAME = my_account_name\nAZURE_BLOB_ACCOUNT_KEY = my_account_key\nAZURE_BLOB_BASE_PATH = /prefix\n\n[lfs]\nAZURE_BLOB_BASE_PATH = /lfs",
			loaders: []any{loadLFSFrom},
			want: []configCheck{
				guard(&LFS.Storage),
				fieldOf("AZURE_BLOB_ACCOUNT_NAME", func() string { return lfsAzure().AccountName }, "my_account_name"),
				fieldOf("AZURE_BLOB_ACCOUNT_KEY", func() string { return lfsAzure().AccountKey }, "my_account_key"),
				fieldOf("AZURE_BLOB_BASE_PATH", func() string { return lfsAzure().BasePath }, "/lfs"),
			},
		},
		{
			name:    "[storage.lfs].AZURE_BLOB_BASE_PATH replaces the prefix",
			ini:     "[storage]\nSTORAGE_TYPE = azureblob\nAZURE_BLOB_ACCOUNT_NAME = my_account_name\nAZURE_BLOB_ACCOUNT_KEY = my_account_key\nAZURE_BLOB_BASE_PATH = /prefix\n\n[storage.lfs]\nAZURE_BLOB_BASE_PATH = /lfs",
			loaders: []any{loadLFSFrom},
			want: []configCheck{
				guard(&LFS.Storage),
				fieldOf("AZURE_BLOB_ACCOUNT_NAME", func() string { return lfsAzure().AccountName }, "my_account_name"),
				fieldOf("AZURE_BLOB_ACCOUNT_KEY", func() string { return lfsAzure().AccountKey }, "my_account_key"),
				fieldOf("AZURE_BLOB_BASE_PATH", func() string { return lfsAzure().BasePath }, "/lfs"),
			},
		},
	})
}
