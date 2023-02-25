// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package history
// This file is the only file that should be changed frequently in this package

var currentGiteaVersion = getVersion("1.19")

// Adds all previously removed settings
// It should declare all breaking configuration changes in chronological order to ensure a monotone increasing error log
func init() {
	MoveSettingInSection("1.6", "api", "ENABLE_SWAGGER_ENDPOINT", "ENABLE_SWAGGER")

	PurgeSettings("1.9", "log.database", "LEVEL", "DRIVER", "CONN")

	MoveSetting("1.12", "markup.sanitizer", "ELEMENT", "markup.sanitizer.1", "ELEMENT")
	MoveSetting("1.12", "markup.sanitizer", "ALLOW_ATTR", "markup.sanitizer.1", "ALLOW_ATTR")
	MoveSetting("1.12", "markup.sanitizer", "REGEXP", "markup.sanitizer.1", "REGEXP")

	PurgeSettings("1.14", "log", "MACARON", "REDIRECT_MACARON_LOG")

	MoveSetting("1.15", "indexer", "ISSUE_INDEXER_QUEUE_TYPE", "queue.issue_indexer", "TYPE")
	MoveSetting("1.15", "indexer", "ISSUE_INDEXER_QUEUE_DIR", "queue.issue_indexer", "DATADIR")
	MoveSetting("1.15", "indexer", "ISSUE_INDEXER_QUEUE_CONN_STR", "queue.issue_indexer", "CONN_STR")
	MoveSetting("1.15", "indexer", "ISSUE_INDEXER_QUEUE_BATCH_NUMBER", "queue.issue_indexer", "BATCH_LENGTH")
	MoveSetting("1.15", "indexer", "UPDATE_BUFFER_LEN", "queue.issue_indexer", "LENGTH")

	MoveSettingInSection("1.17", "cron.archive_cleanup", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.update_mirrors", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.repo_health_check", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.check_repo_stats", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.update_migration_poster_id", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.sync_external_users", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.deleted_branches_cleanup", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_inactive_accounts", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_repo_archives", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.git_gc_repos", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.resync_all_sshkeys", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.resync_all_hooks", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.reinit_missing_repos", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_missing_repos", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_generated_repository_avatars", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")
	MoveSettingInSection("1.17", "cron.delete_old_actions", "NO_SUCCESS_NOTICE", "NOTIFY_ON_SUCCESS")

	PurgeSettings("1.18", "U2F", "APP_ID")
	MoveSettingsToDB("1.18", "picture", "ENABLE_FEDERATED_AVATAR", "DISABLE_GRAVATAR")
	MoveSettingInSection("1.18", "mailer", "HOST", "SMTP_ADDR+SMTP_PORT")
	MoveSettingInSection("1.18", "mailer", "MAILER_TYPE", "PROTOCOL")
	MoveSettingInSection("1.18", "mailer", "IS_TLS_ENABLED", "PROTOCOL")
	MoveSettingInSection("1.18", "mailer", "DISABLE_HELO", "ENABLE_HELO")
	MoveSettingInSection("1.18", "mailer", "SKIP_VERIFY", "FORCE_TRUST_SERVER_CERT")
	MoveSettingInSection("1.18", "mailer", "USE_CERTIFICATE", "USE_CLIENT_CERT")
	MoveSettingInSection("1.18", "mailer", "CERT_FILE", "CLIENT_CERT_FILE")
	MoveSettingInSection("1.18", "mailer", "KEY_FILE", "CLIENT_KEY_FILE")

	PurgeSettings("1.19", "ui", "ONLY_SHOW_RELEVANT_REPOS")
}
