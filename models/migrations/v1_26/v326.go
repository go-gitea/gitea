// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

// Migration 326 used to rewrite legacy Actions commit status target URLs
// from run/job indexes to run/job IDs.
//
// The original migration required scanning and updating large commit status
// tables, which could make upgrades unacceptably slow on large instances.
// Compatibility is now handled in the web Actions routes by best-effort
// redirects from legacy index-based URLs to ID-based URLs on demand, instead
// of strictly rewriting every legacy target URL in the database.
//
// Administrators who need to rewrite legacy target URLs explicitly can run the
// "fix-commit-status-target-url" doctor check.
//
// The migration itself is registered as a no-op in models/migrations/migrations.go
// to preserve the existing migration sequence.
