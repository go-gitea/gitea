// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func Test_getStorageInheritNameSectionTypeForActions(t *testing.T) {
	defer test.MockVariableValue(&Actions)()

	logType := func() StorageType { return Actions.LogStorage.Type }
	logBasePath := func() string { return Actions.LogStorage.MinioConfig.BasePath }
	logDir := func() string { return filepath.Base(Actions.LogStorage.Path) }
	artifactType := func() StorageType { return Actions.ArtifactStorage.Type }
	artifactBasePath := func() string { return Actions.ArtifactStorage.MinioConfig.BasePath }
	artifactDir := func() string { return filepath.Base(Actions.ArtifactStorage.Path) }

	testConfigLoad(t, []any{loadActionsFrom}, []configTestCase{
		{
			name: "both inherit the global [storage]",
			ini:  "[storage]\nSTORAGE_TYPE = minio",
			want: []configCheck{
				fieldOf("STORAGE_TYPE", logType, MinioStorageType),
				fieldOf("MINIO_BASE_PATH", logBasePath, "actions_log/"),
				fieldOf("STORAGE_TYPE", artifactType, MinioStorageType),
				fieldOf("MINIO_BASE_PATH", artifactBasePath, "actions_artifacts/"),
			},
		},
		{
			name: "[storage.actions_log] only affects the log storage",
			ini:  "[storage.actions_log]\nSTORAGE_TYPE = minio",
			want: []configCheck{
				fieldOf("STORAGE_TYPE", logType, MinioStorageType),
				fieldOf("MINIO_BASE_PATH", logBasePath, "actions_log/"),
				fieldOf("STORAGE_TYPE", artifactType, LocalStorageType),
				fieldOf("PATH", artifactDir, "actions_artifacts"),
			},
		},
		{
			name: "the log storage can name another storage",
			ini:  "[storage.actions_log]\nSTORAGE_TYPE = my_storage\n\n[storage.my_storage]\nSTORAGE_TYPE = minio",
			want: []configCheck{
				fieldOf("STORAGE_TYPE", logType, MinioStorageType),
				fieldOf("MINIO_BASE_PATH", logBasePath, "actions_log/"),
				fieldOf("STORAGE_TYPE", artifactType, LocalStorageType),
				fieldOf("PATH", artifactDir, "actions_artifacts"),
			},
		},
		{
			name: "the artifact storage can name another storage",
			ini:  "[storage.actions_artifacts]\nSTORAGE_TYPE = my_storage\n\n[storage.my_storage]\nSTORAGE_TYPE = minio",
			want: []configCheck{
				fieldOf("STORAGE_TYPE", logType, LocalStorageType),
				fieldOf("PATH", logDir, "actions_log"),
				fieldOf("STORAGE_TYPE", artifactType, MinioStorageType),
				fieldOf("MINIO_BASE_PATH", artifactBasePath, "actions_artifacts/"),
			},
		},
		{
			name: "both default to local",
			want: []configCheck{
				fieldOf("STORAGE_TYPE", logType, LocalStorageType),
				fieldOf("PATH", logDir, "actions_log"),
				fieldOf("STORAGE_TYPE", artifactType, LocalStorageType),
				fieldOf("PATH", artifactDir, "actions_artifacts"),
			},
		},
	})
}

func Test_WorkflowDirs(t *testing.T) {
	defer test.MockVariableValue(&Actions)()

	testConfigLoad(t, []any{loadActionsFrom}, []configTestCase{
		{
			name: "default",
			ini:  "[actions]",
			want: []configCheck{field("WORKFLOW_DIRS", &Actions.WorkflowDirs, []string{".gitea/workflows", ".github/workflows"})},
		},
		{
			name: "single dir",
			ini:  "[actions]\nWORKFLOW_DIRS = .github/workflows",
			want: []configCheck{field("WORKFLOW_DIRS", &Actions.WorkflowDirs, []string{".github/workflows"})},
		},
		{
			name: "custom order",
			ini:  "[actions]\nWORKFLOW_DIRS = .github/workflows,.gitea/workflows",
			want: []configCheck{field("WORKFLOW_DIRS", &Actions.WorkflowDirs, []string{".github/workflows", ".gitea/workflows"})},
		},
		{
			name: "whitespace trimming",
			ini:  "[actions]\nWORKFLOW_DIRS = .gitea/workflows , .github/workflows ",
			want: []configCheck{field("WORKFLOW_DIRS", &Actions.WorkflowDirs, []string{".gitea/workflows", ".github/workflows"})},
		},
		{
			name: "trailing slash normalization",
			ini:  "[actions]\nWORKFLOW_DIRS = .gitea/workflows/,.github/workflows/",
			want: []configCheck{field("WORKFLOW_DIRS", &Actions.WorkflowDirs, []string{".gitea/workflows", ".github/workflows"})},
		},
		{
			name:    "only commas and whitespace",
			ini:     "[actions]\nWORKFLOW_DIRS = , , ,",
			wantErr: assert.Error,
			want:    []configCheck{guard(&Actions.WorkflowDirs)},
		},
	})
}

func Test_getDefaultActionsURLForActions(t *testing.T) {
	defer test.MockVariableValue(&Actions)()
	defer test.MockVariableValue(&AppURL, "http://test_get_default_actions_url_for_actions:3000/")()

	actionsURL := func() string { return Actions.DefaultActionsURL.URL() }

	testConfigLoad(t, []any{loadActionsFrom}, []configTestCase{
		{
			name: "default",
			ini:  "[actions]",
			want: []configCheck{fieldOf("DEFAULT_ACTIONS_URL", actionsURL, "https://github.com")},
		},
		{
			name: "github",
			ini:  "[actions]\nDEFAULT_ACTIONS_URL = github",
			want: []configCheck{fieldOf("DEFAULT_ACTIONS_URL", actionsURL, "https://github.com")},
		},
		{
			name: "self",
			ini:  "[actions]\nDEFAULT_ACTIONS_URL = self",
			want: []configCheck{fieldOf("DEFAULT_ACTIONS_URL", actionsURL, "http://test_get_default_actions_url_for_actions:3000")},
		},
		{
			name: "custom url falls back to github",
			ini:  "[actions]\nDEFAULT_ACTIONS_URL = https://gitea.com",
			want: []configCheck{fieldOf("DEFAULT_ACTIONS_URL", actionsURL, "https://github.com")},
		},
		{
			name: "custom urls fall back to github",
			ini:  "[actions]\nDEFAULT_ACTIONS_URL = https://gitea.com,https://github.com",
			want: []configCheck{fieldOf("DEFAULT_ACTIONS_URL", actionsURL, "https://github.com")},
		},
		{
			name:    "invalid",
			ini:     "[actions]\nDEFAULT_ACTIONS_URL = gitea",
			wantErr: assert.Error,
			want: []configCheck{
				guard(&Actions.DefaultActionsURL),
				fieldOf("DEFAULT_ACTIONS_URL", actionsURL, "https://github.com"),
			},
		},
	})
}

func Test_ScopedWorkflowDirs(t *testing.T) {
	defer test.MockVariableValue(&Actions)()

	// WORKFLOW_DIRS is guarded rather than asserted: MapTo keeps the current value for an absent
	// key, so without it a case that sets WORKFLOW_DIRS would leak into the next one
	scoped := func(want []string) []configCheck {
		return []configCheck{
			guard(&Actions.WorkflowDirs),
			field("SCOPED_WORKFLOW_DIRS", &Actions.ScopedWorkflowDirs, want),
		}
	}
	overlapping := []configCheck{guard(&Actions.WorkflowDirs), guard(&Actions.ScopedWorkflowDirs)}

	testConfigLoad(t, []any{loadActionsFrom}, []configTestCase{
		{
			name: "default",
			ini:  "[actions]",
			want: scoped([]string{".gitea/scoped_workflows"}),
		},
		{
			name: "custom dir",
			ini:  "[actions]\nSCOPED_WORKFLOW_DIRS = .gitea/my-scoped",
			want: scoped([]string{".gitea/my-scoped"}),
		},
		{
			name: "empty disables the feature",
			ini:  "[actions]\nSCOPED_WORKFLOW_DIRS = , ,",
			want: scoped([]string{}),
		},
		{
			name:    "overlap equal with workflow dir",
			ini:     "[actions]\nWORKFLOW_DIRS = .gitea/workflows\nSCOPED_WORKFLOW_DIRS = .gitea/workflows",
			wantErr: assert.Error,
			want:    overlapping,
		},
		{
			name:    "scoped dir nested under workflow dir",
			ini:     "[actions]\nWORKFLOW_DIRS = .gitea/workflows\nSCOPED_WORKFLOW_DIRS = .gitea/workflows/scoped",
			wantErr: assert.Error,
			want:    overlapping,
		},
		{
			name:    "workflow dir nested under scoped dir",
			ini:     "[actions]\nWORKFLOW_DIRS = .gitea/workflows/ci\nSCOPED_WORKFLOW_DIRS = .gitea/workflows",
			wantErr: assert.Error,
			want:    overlapping,
		},
		{
			name: "no overlap",
			ini:  "[actions]\nSCOPED_WORKFLOW_DIRS = .gitea/scoped",
			want: scoped([]string{".gitea/scoped"}),
		},
	})
}
