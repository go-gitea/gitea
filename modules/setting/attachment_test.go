// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"slices"
	"testing"

	"gitea.dev/modules/test"
)

func Test_getStorageAttachment(t *testing.T) {
	defer test.MockVariableValue(&Attachment)()

	testConfigLoad(t, []any{loadAttachmentFrom}, []configTestCase{
		{
			name: "a custom named storage keeps the section bucket",
			ini: `
[attachment]
STORAGE_TYPE = my_minio
MINIO_BUCKET = gitea-attachment

[storage.my_minio]
STORAGE_TYPE = minio
MINIO_ENDPOINT = my_minio:9000
`,
			want: slices.Concat(
				minioStorage("attachment", &Attachment.Storage, "gitea-attachment", "attachments/"),
				[]configCheck{fieldOf("MINIO_ENDPOINT", func() string { return Attachment.Storage.MinioConfig.Endpoint }, "my_minio:9000")},
			),
		},
		{
			name: "[storage.minio] overrides [storage]",
			ini: `
[attachment]
STORAGE_TYPE = minio

[storage.minio]
MINIO_BUCKET = gitea-minio

[storage]
MINIO_BUCKET = gitea
`,
			want: minioStorage("attachment", &Attachment.Storage, "gitea-minio", "attachments/"),
		},
		{
			name: "the [attachment] section wins over [storage.attachments]",
			ini: `
[attachment]
STORAGE_TYPE = minio
MINIO_BUCKET = gitea-attachment

[storage.attachments]
MINIO_BUCKET = gitea

[storage]
STORAGE_TYPE = local
`,
			want: minioStorage("attachment", &Attachment.Storage, "gitea-attachment", "attachments/"),
		},
		{
			name: "the default storage is local, so the bucket stays empty",
			want: []configCheck{
				guard(&Attachment.Storage),
				fieldOf("MINIO_BUCKET", func() string { return Attachment.Storage.MinioConfig.Bucket }, ""),
			},
		},
		{
			name: "[storage.attachments] alone sets the type",
			ini:  "[storage.attachments]\nSTORAGE_TYPE = minio",
			want: []configCheck{
				guard(&Attachment.Storage),
				fieldOf("STORAGE_TYPE", func() StorageType { return Attachment.Storage.Type }, MinioStorageType),
			},
		},
		{
			name: "a fully configured [storage] section",
			ini: `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage]
STORAGE_TYPE            = minio
MINIO_ENDPOINT          = s3.my-domain.net
MINIO_BUCKET            = gitea
MINIO_LOCATION          = homenet
MINIO_USE_SSL           = true
MINIO_ACCESS_KEY_ID     = correct_key
MINIO_SECRET_ACCESS_KEY = correct_key
`,
			want: []configCheck{
				guard(&Attachment.Storage),
				fieldOf("STORAGE_TYPE", func() StorageType { return Attachment.Storage.Type }, MinioStorageType),
				fieldOf("MINIO_BUCKET", func() string { return Attachment.Storage.MinioConfig.Bucket }, "gitea"),
			},
		},
		{
			name: "the global [storage] type is inherited",
			ini:  "[storage]\nSTORAGE_TYPE = minio",
			want: minioStorage("attachment", &Attachment.Storage, "gitea", "attachments/"),
		},
	})
}
