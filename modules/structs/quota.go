// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// StorageQuotaOptions replaces a subject's overrides. Null or omitted limits inherit the configured defaults.
type StorageQuotaOptions struct {
	GitLimit   *int64 `json:"git_limit"`
	LFSLimit   *int64 `json:"lfs_limit"`
	TotalLimit *int64 `json:"total_limit"`
}

type StorageQuotaResource struct {
	Used     int64 `json:"used"`
	Limit    int64 `json:"limit"`
	Reserved int64 `json:"reserved"`
	Warning  bool  `json:"warning"`
}

type StorageQuota struct {
	Subject   string                `json:"subject"`
	SubjectID int64                 `json:"subject_id"`
	Name      string                `json:"name"`
	Overrides StorageQuotaOptions   `json:"overrides"`
	Git       *StorageQuotaResource `json:"git,omitempty"`
	LFS       *StorageQuotaResource `json:"lfs,omitempty"`
	Total     *StorageQuotaResource `json:"total,omitempty"`
	Account   *StorageQuotaResource `json:"account,omitempty"`
}
