// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package variable

import (
	"strings"

	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/terror"
)

// ScopeFlag is for system variable whether can be changed in global/session dynamically or not.
type ScopeFlag uint8

const (
	// ScopeNone means the system variable can not be changed dynamically.
	ScopeNone ScopeFlag = iota << 0
	// ScopeGlobal means the system variable can be changed globally.
	ScopeGlobal
	// ScopeSession means the system variable can only be changed in current session.
	ScopeSession
)

// SysVar is for system variable.
type SysVar struct {
	// Scope is for whether can be changed or not
	Scope ScopeFlag

	// Variable name
	Name string

	// Variable value
	Value string
}

// SysVars is global sys vars map.
var SysVars map[string]*SysVar

// GetSysVar returns sys var info for name as key.
func GetSysVar(name string) *SysVar {
	name = strings.ToLower(name)
	return SysVars[name]
}

// Variable error codes.
const (
	CodeUnknownStatusVar terror.ErrCode = 1
	CodeUnknownSystemVar terror.ErrCode = 1193
)

// Variable errors
var (
	UnknownStatusVar = terror.ClassVariable.New(CodeUnknownStatusVar, "unknown status variable")
	UnknownSystemVar = terror.ClassVariable.New(CodeUnknownSystemVar, "unknown system variable")
)

func init() {
	SysVars = make(map[string]*SysVar)
	for _, v := range defaultSysVars {
		SysVars[v.Name] = v
	}

	// Register terror to mysql error map.
	mySQLErrCodes := map[terror.ErrCode]uint16{
		CodeUnknownSystemVar: mysql.ErrUnknownSystemVariable,
	}
	terror.ErrClassToMySQLCodes[terror.ClassVariable] = mySQLErrCodes
}

// we only support MySQL now
var defaultSysVars = []*SysVar{
	{ScopeGlobal, "gtid_mode", "OFF"},
	{ScopeGlobal, "flush_time", "0"},
	{ScopeSession, "pseudo_slave_mode", ""},
	{ScopeNone, "performance_schema_max_mutex_classes", "200"},
	{ScopeGlobal | ScopeSession, "low_priority_updates", "OFF"},
	{ScopeGlobal | ScopeSession, "session_track_gtids", ""},
	{ScopeGlobal | ScopeSession, "ndbinfo_max_rows", ""},
	{ScopeGlobal | ScopeSession, "ndb_index_stat_option", ""},
	{ScopeGlobal | ScopeSession, "old_passwords", "0"},
	{ScopeNone, "innodb_version", "5.6.25"},
	{ScopeGlobal, "max_connections", "151"},
	{ScopeGlobal | ScopeSession, "big_tables", "OFF"},
	{ScopeNone, "skip_external_locking", "ON"},
	{ScopeGlobal, "slave_pending_jobs_size_max", "16777216"},
	{ScopeNone, "innodb_sync_array_size", "1"},
	{ScopeSession, "rand_seed2", ""},
	{ScopeGlobal, "validate_password_number_count", ""},
	{ScopeSession, "gtid_next", ""},
	{ScopeGlobal | ScopeSession, "sql_select_limit", "18446744073709551615"},
	{ScopeGlobal, "ndb_show_foreign_key_mock_tables", ""},
	{ScopeNone, "multi_range_count", "256"},
	{ScopeGlobal | ScopeSession, "default_week_format", "0"},
	{ScopeGlobal | ScopeSession, "binlog_error_action", "IGNORE_ERROR"},
	{ScopeGlobal, "slave_transaction_retries", "10"},
	{ScopeGlobal | ScopeSession, "default_storage_engine", "InnoDB"},
	{ScopeNone, "ft_query_expansion_limit", "20"},
	{ScopeGlobal, "max_connect_errors", "100"},
	{ScopeGlobal, "sync_binlog", "0"},
	{ScopeNone, "max_digest_length", "1024"},
	{ScopeNone, "innodb_force_load_corrupted", "OFF"},
	{ScopeNone, "performance_schema_max_table_handles", "4000"},
	{ScopeGlobal, "innodb_fast_shutdown", "1"},
	{ScopeNone, "ft_max_word_len", "84"},
	{ScopeGlobal, "log_backward_compatible_user_definitions", ""},
	{ScopeNone, "lc_messages_dir", "/usr/local/mysql-5.6.25-osx10.8-x86_64/share/"},
	{ScopeGlobal, "ft_boolean_syntax", "+ -><()~*:\"\"&|"},
	{ScopeGlobal, "table_definition_cache", "1400"},
	{ScopeNone, "skip_name_resolve", "OFF"},
	{ScopeNone, "performance_schema_max_file_handles", "32768"},
	{ScopeSession, "transaction_allow_batching", ""},
	{ScopeGlobal | ScopeSession, "sql_mode", "NO_ENGINE_SUBSTITUTION"},
	{ScopeNone, "performance_schema_max_statement_classes", "168"},
	{ScopeGlobal, "server_id", "0"},
	{ScopeGlobal, "innodb_flushing_avg_loops", "30"},
	{ScopeGlobal | ScopeSession, "tmp_table_size", "16777216"},
	{ScopeGlobal, "innodb_max_purge_lag", "0"},
	{ScopeGlobal | ScopeSession, "preload_buffer_size", "32768"},
	{ScopeGlobal, "slave_checkpoint_period", "300"},
	{ScopeGlobal, "check_proxy_users", ""},
	{ScopeNone, "have_query_cache", "YES"},
	{ScopeGlobal, "innodb_flush_log_at_timeout", "1"},
	{ScopeGlobal, "innodb_max_undo_log_size", ""},
	{ScopeGlobal | ScopeSession, "range_alloc_block_size", "4096"},
	{ScopeGlobal, "connect_timeout", "10"},
	{ScopeGlobal | ScopeSession, "collation_server", "latin1_swedish_ci"},
	{ScopeNone, "have_rtree_keys", "YES"},
	{ScopeGlobal, "innodb_old_blocks_pct", "37"},
	{ScopeGlobal, "innodb_file_format", "Antelope"},
	{ScopeGlobal, "innodb_compression_failure_threshold_pct", "5"},
	{ScopeNone, "performance_schema_events_waits_history_long_size", "10000"},
	{ScopeGlobal, "innodb_checksum_algorithm", "innodb"},
	{ScopeNone, "innodb_ft_sort_pll_degree", "2"},
	{ScopeNone, "thread_stack", "262144"},
	{ScopeGlobal, "relay_log_info_repository", "FILE"},
	{ScopeGlobal | ScopeSession, "sql_log_bin", "ON"},
	{ScopeGlobal, "super_read_only", ""},
	{ScopeGlobal | ScopeSession, "max_delayed_threads", "20"},
	{ScopeNone, "protocol_version", "10"},
	{ScopeGlobal | ScopeSession, "new", "OFF"},
	{ScopeGlobal | ScopeSession, "myisam_sort_buffer_size", "8388608"},
	{ScopeGlobal | ScopeSession, "optimizer_trace_offset", "-1"},
	{ScopeGlobal, "innodb_buffer_pool_dump_at_shutdown", "OFF"},
	{ScopeGlobal | ScopeSession, "sql_notes", "ON"},
	{ScopeGlobal, "innodb_cmp_per_index_enabled", "OFF"},
	{ScopeGlobal, "innodb_ft_server_stopword_table", ""},
	{ScopeNone, "performance_schema_max_file_instances", "7693"},
	{ScopeNone, "log_output", "FILE"},
	{ScopeGlobal, "binlog_group_commit_sync_delay", ""},
	{ScopeGlobal, "binlog_group_commit_sync_no_delay_count", ""},
	{ScopeNone, "have_crypt", "YES"},
	{ScopeGlobal, "innodb_log_write_ahead_size", ""},
	{ScopeNone, "innodb_log_group_home_dir", "./"},
	{ScopeNone, "performance_schema_events_statements_history_size", "10"},
	{ScopeGlobal, "general_log", "OFF"},
	{ScopeGlobal, "validate_password_dictionary_file", ""},
	{ScopeGlobal, "binlog_order_commits", "ON"},
	{ScopeGlobal, "master_verify_checksum", "OFF"},
	{ScopeGlobal, "key_cache_division_limit", "100"},
	{ScopeGlobal, "rpl_semi_sync_master_trace_level", ""},
	{ScopeGlobal | ScopeSession, "max_insert_delayed_threads", "20"},
	{ScopeNone, "performance_schema_session_connect_attrs_size", "512"},
	{ScopeGlobal | ScopeSession, "time_zone", "SYSTEM"},
	{ScopeGlobal, "innodb_max_dirty_pages_pct", "75"},
	{ScopeGlobal, "innodb_file_per_table", "ON"},
	{ScopeGlobal, "innodb_log_compressed_pages", "ON"},
	{ScopeGlobal, "master_info_repository", "FILE"},
	{ScopeGlobal, "rpl_stop_slave_timeout", "31536000"},
	{ScopeNone, "skip_networking", "OFF"},
	{ScopeGlobal, "innodb_monitor_reset", ""},
	{ScopeNone, "have_ssl", "DISABLED"},
	{ScopeNone, "system_time_zone", "CST"},
	{ScopeGlobal, "innodb_print_all_deadlocks", "OFF"},
	{ScopeNone, "innodb_autoinc_lock_mode", "1"},
	{ScopeGlobal, "slave_net_timeout", "3600"},
	{ScopeGlobal, "key_buffer_size", "8388608"},
	{ScopeGlobal | ScopeSession, "foreign_key_checks", "ON"},
	{ScopeGlobal, "host_cache_size", "279"},
	{ScopeGlobal, "delay_key_write", "ON"},
	{ScopeNone, "metadata_locks_cache_size", "1024"},
	{ScopeNone, "innodb_force_recovery", "0"},
	{ScopeGlobal, "innodb_file_format_max", "Antelope"},
	{ScopeGlobal | ScopeSession, "debug", ""},
	{ScopeGlobal, "log_warnings", "1"},
	{ScopeGlobal, "offline_mode", ""},
	{ScopeGlobal | ScopeSession, "innodb_strict_mode", "OFF"},
	{ScopeGlobal, "innodb_rollback_segments", "128"},
	{ScopeGlobal | ScopeSession, "join_buffer_size", "262144"},
	{ScopeNone, "innodb_mirrored_log_groups", "1"},
	{ScopeGlobal, "max_binlog_size", "1073741824"},
	{ScopeGlobal, "sync_master_info", "10000"},
	{ScopeGlobal, "concurrent_insert", "AUTO"},
	{ScopeGlobal, "innodb_adaptive_hash_index", "ON"},
	{ScopeGlobal, "innodb_ft_enable_stopword", "ON"},
	{ScopeGlobal, "general_log_file", "/usr/local/mysql/data/localhost.log"},
	{ScopeGlobal | ScopeSession, "innodb_support_xa", "ON"},
	{ScopeGlobal, "innodb_compression_level", "6"},
	{ScopeNone, "innodb_file_format_check", "ON"},
	{ScopeNone, "myisam_mmap_size", "18446744073709551615"},
	{ScopeGlobal, "init_slave", ""},
	{ScopeNone, "innodb_buffer_pool_instances", "8"},
	{ScopeGlobal | ScopeSession, "block_encryption_mode", "aes-128-ecb"},
	{ScopeGlobal | ScopeSession, "max_length_for_sort_data", "1024"},
	{ScopeNone, "character_set_system", "utf8"},
	{ScopeGlobal | ScopeSession, "interactive_timeout", "28800"},
	{ScopeGlobal, "innodb_optimize_fulltext_only", "OFF"},
	{ScopeNone, "character_sets_dir", "/usr/local/mysql-5.6.25-osx10.8-x86_64/share/charsets/"},
	{ScopeGlobal | ScopeSession, "query_cache_type", "OFF"},
	{ScopeNone, "innodb_rollback_on_timeout", "OFF"},
	{ScopeGlobal | ScopeSession, "query_alloc_block_size", "8192"},
	{ScopeGlobal, "slave_compressed_protocol", "OFF"},
	{ScopeGlobal, "init_connect", ""},
	{ScopeGlobal, "rpl_semi_sync_slave_trace_level", ""},
	{ScopeNone, "have_compress", "YES"},
	{ScopeNone, "thread_concurrency", "10"},
	{ScopeGlobal | ScopeSession, "query_prealloc_size", "8192"},
	{ScopeNone, "relay_log_space_limit", "0"},
	{ScopeGlobal | ScopeSession, "max_user_connections", "0"},
	{ScopeNone, "performance_schema_max_thread_classes", "50"},
	{ScopeGlobal, "innodb_api_trx_level", "0"},
	{ScopeNone, "disconnect_on_expired_password", "ON"},
	{ScopeNone, "performance_schema_max_file_classes", "50"},
	{ScopeGlobal, "expire_logs_days", "0"},
	{ScopeGlobal | ScopeSession, "binlog_rows_query_log_events", "OFF"},
	{ScopeGlobal, "validate_password_policy", ""},
	{ScopeGlobal, "default_password_lifetime", ""},
	{ScopeNone, "pid_file", "/usr/local/mysql/data/localhost.pid"},
	{ScopeNone, "innodb_undo_tablespaces", "0"},
	{ScopeGlobal, "innodb_status_output_locks", "OFF"},
	{ScopeNone, "performance_schema_accounts_size", "100"},
	{ScopeGlobal | ScopeSession, "max_error_count", "64"},
	{ScopeGlobal, "max_write_lock_count", "18446744073709551615"},
	{ScopeNone, "performance_schema_max_socket_instances", "322"},
	{ScopeNone, "performance_schema_max_table_instances", "12500"},
	{ScopeGlobal, "innodb_stats_persistent_sample_pages", "20"},
	{ScopeGlobal, "show_compatibility_56", ""},
	{ScopeGlobal, "log_slow_slave_statements", "OFF"},
	{ScopeNone, "innodb_open_files", "2000"},
	{ScopeGlobal, "innodb_spin_wait_delay", "6"},
	{ScopeGlobal, "thread_cache_size", "9"},
	{ScopeGlobal, "log_slow_admin_statements", "OFF"},
	{ScopeNone, "innodb_checksums", "ON"},
	{ScopeNone, "hostname", "localhost"},
	{ScopeGlobal | ScopeSession, "auto_increment_offset", "1"},
	{ScopeNone, "ft_stopword_file", "(built-in)"},
	{ScopeGlobal, "innodb_max_dirty_pages_pct_lwm", "0"},
	{ScopeGlobal, "log_queries_not_using_indexes", "OFF"},
	{ScopeSession, "timestamp", ""},
	{ScopeGlobal | ScopeSession, "query_cache_wlock_invalidate", "OFF"},
	{ScopeGlobal | ScopeSession, "sql_buffer_result", "OFF"},
	{ScopeGlobal | ScopeSession, "character_set_filesystem", "binary"},
	{ScopeGlobal | ScopeSession, "collation_database", "latin1_swedish_ci"},
	{ScopeGlobal | ScopeSession, "auto_increment_increment", "1"},
	{ScopeGlobal | ScopeSession, "max_heap_table_size", "16777216"},
	{ScopeGlobal | ScopeSession, "div_precision_increment", "4"},
	{ScopeGlobal, "innodb_lru_scan_depth", "1024"},
	{ScopeGlobal, "innodb_purge_rseg_truncate_frequency", ""},
	{ScopeGlobal | ScopeSession, "sql_auto_is_null", "OFF"},
	{ScopeNone, "innodb_api_enable_binlog", "OFF"},
	{ScopeGlobal | ScopeSession, "innodb_ft_user_stopword_table", ""},
	{ScopeNone, "server_id_bits", "32"},
	{ScopeGlobal, "innodb_log_checksum_algorithm", ""},
	{ScopeNone, "innodb_buffer_pool_load_at_startup", "OFF"},
	{ScopeGlobal | ScopeSession, "sort_buffer_size", "262144"},
	{ScopeGlobal, "innodb_flush_neighbors", "1"},
	{ScopeNone, "innodb_use_sys_malloc", "ON"},
	{ScopeNone, "plugin_dir", "/usr/local/mysql/lib/plugin/"},
	{ScopeNone, "performance_schema_max_socket_classes", "10"},
	{ScopeNone, "performance_schema_max_stage_classes", "150"},
	{ScopeGlobal, "innodb_purge_batch_size", "300"},
	{ScopeNone, "have_profiling", "YES"},
	{ScopeGlobal, "slave_checkpoint_group", "512"},
	{ScopeGlobal | ScopeSession, "character_set_client", "latin1"},
	{ScopeNone, "slave_load_tmpdir", "/var/tmp/"},
	{ScopeGlobal, "innodb_buffer_pool_dump_now", "OFF"},
	{ScopeGlobal, "relay_log_purge", "ON"},
	{ScopeGlobal, "ndb_distribution", ""},
	{ScopeGlobal, "myisam_data_pointer_size", "6"},
	{ScopeGlobal, "ndb_optimization_delay", ""},
	{ScopeGlobal, "innodb_ft_num_word_optimize", "2000"},
	{ScopeGlobal | ScopeSession, "max_join_size", "18446744073709551615"},
	{ScopeNone, "core_file", "OFF"},
	{ScopeGlobal | ScopeSession, "max_seeks_for_key", "18446744073709551615"},
	{ScopeNone, "innodb_log_buffer_size", "8388608"},
	{ScopeGlobal, "delayed_insert_timeout", "300"},
	{ScopeGlobal, "max_relay_log_size", "0"},
	{ScopeGlobal | ScopeSession, "max_sort_length", "1024"},
	{ScopeNone, "metadata_locks_hash_instances", "8"},
	{ScopeGlobal, "ndb_eventbuffer_free_percent", ""},
	{ScopeNone, "large_files_support", "ON"},
	{ScopeGlobal, "binlog_max_flush_queue_time", "0"},
	{ScopeGlobal, "innodb_fill_factor", ""},
	{ScopeGlobal, "log_syslog_facility", ""},
	{ScopeNone, "innodb_ft_min_token_size", "3"},
	{ScopeGlobal | ScopeSession, "transaction_write_set_extraction", ""},
	{ScopeGlobal | ScopeSession, "ndb_blob_write_batch_bytes", ""},
	{ScopeGlobal, "automatic_sp_privileges", "ON"},
	{ScopeGlobal, "innodb_flush_sync", ""},
	{ScopeNone, "performance_schema_events_statements_history_long_size", "10000"},
	{ScopeGlobal, "innodb_monitor_disable", ""},
	{ScopeNone, "innodb_doublewrite", "ON"},
	{ScopeGlobal, "slave_parallel_type", ""},
	{ScopeNone, "log_bin_use_v1_row_events", "OFF"},
	{ScopeSession, "innodb_optimize_point_storage", ""},
	{ScopeNone, "innodb_api_disable_rowlock", "OFF"},
	{ScopeGlobal, "innodb_adaptive_flushing_lwm", "10"},
	{ScopeNone, "innodb_log_files_in_group", "2"},
	{ScopeGlobal, "innodb_buffer_pool_load_now", "OFF"},
	{ScopeNone, "performance_schema_max_rwlock_classes", "40"},
	{ScopeNone, "binlog_gtid_simple_recovery", "OFF"},
	{ScopeNone, "port", "3306"},
	{ScopeNone, "performance_schema_digests_size", "10000"},
	{ScopeGlobal | ScopeSession, "profiling", "OFF"},
	{ScopeNone, "lower_case_table_names", "2"},
	{ScopeSession, "rand_seed1", ""},
	{ScopeGlobal, "sha256_password_proxy_users", ""},
	{ScopeGlobal | ScopeSession, "sql_quote_show_create", "ON"},
	{ScopeGlobal | ScopeSession, "binlogging_impossible_mode", "IGNORE_ERROR"},
	{ScopeGlobal, "query_cache_size", "1048576"},
	{ScopeGlobal, "innodb_stats_transient_sample_pages", "8"},
	{ScopeGlobal, "innodb_stats_on_metadata", "OFF"},
	{ScopeNone, "server_uuid", "d530594e-1c86-11e5-878b-6b36ce6799ca"},
	{ScopeNone, "open_files_limit", "5000"},
	{ScopeGlobal | ScopeSession, "ndb_force_send", ""},
	{ScopeNone, "skip_show_database", "OFF"},
	{ScopeGlobal, "log_timestamps", ""},
	{ScopeNone, "version_compile_machine", "x86_64"},
	{ScopeGlobal, "slave_parallel_workers", "0"},
	{ScopeGlobal, "event_scheduler", "OFF"},
	{ScopeGlobal | ScopeSession, "ndb_deferred_constraints", ""},
	{ScopeGlobal, "log_syslog_include_pid", ""},
	{ScopeSession, "last_insert_id", ""},
	{ScopeNone, "innodb_ft_cache_size", "8000000"},
	{ScopeNone, "log_bin", "OFF"},
	{ScopeGlobal, "innodb_disable_sort_file_cache", "OFF"},
	{ScopeGlobal, "log_error_verbosity", ""},
	{ScopeNone, "performance_schema_hosts_size", "100"},
	{ScopeGlobal, "innodb_replication_delay", "0"},
	{ScopeGlobal, "slow_query_log", "OFF"},
	{ScopeSession, "debug_sync", ""},
	{ScopeGlobal, "innodb_stats_auto_recalc", "ON"},
	{ScopeGlobal, "timed_mutexes", "OFF"},
	{ScopeGlobal | ScopeSession, "lc_messages", "en_US"},
	{ScopeGlobal | ScopeSession, "bulk_insert_buffer_size", "8388608"},
	{ScopeGlobal | ScopeSession, "binlog_direct_non_transactional_updates", "OFF"},
	{ScopeGlobal, "innodb_change_buffering", "all"},
	{ScopeGlobal | ScopeSession, "sql_big_selects", "ON"},
	{ScopeGlobal | ScopeSession, "character_set_results", "latin1"},
	{ScopeGlobal, "innodb_max_purge_lag_delay", "0"},
	{ScopeGlobal | ScopeSession, "session_track_schema", ""},
	{ScopeGlobal, "innodb_io_capacity_max", "2000"},
	{ScopeGlobal, "innodb_autoextend_increment", "64"},
	{ScopeGlobal | ScopeSession, "binlog_format", "STATEMENT"},
	{ScopeGlobal | ScopeSession, "optimizer_trace", "enabled=off,one_line=off"},
	{ScopeGlobal | ScopeSession, "read_rnd_buffer_size", "262144"},
	{ScopeNone, "version_comment", "MySQL Community Server (GPL)"},
	{ScopeGlobal | ScopeSession, "net_write_timeout", "60"},
	{ScopeGlobal, "innodb_buffer_pool_load_abort", "OFF"},
	{ScopeGlobal | ScopeSession, "tx_isolation", "REPEATABLE-READ"},
	{ScopeGlobal | ScopeSession, "collation_connection", "latin1_swedish_ci"},
	{ScopeGlobal, "rpl_semi_sync_master_timeout", ""},
	{ScopeGlobal | ScopeSession, "transaction_prealloc_size", "4096"},
	{ScopeNone, "slave_skip_errors", "OFF"},
	{ScopeNone, "performance_schema_setup_objects_size", "100"},
	{ScopeGlobal, "sync_relay_log", "10000"},
	{ScopeGlobal, "innodb_ft_result_cache_limit", "2000000000"},
	{ScopeNone, "innodb_sort_buffer_size", "1048576"},
	{ScopeGlobal, "innodb_ft_enable_diag_print", "OFF"},
	{ScopeNone, "thread_handling", "one-thread-per-connection"},
	{ScopeGlobal, "stored_program_cache", "256"},
	{ScopeNone, "performance_schema_max_mutex_instances", "15906"},
	{ScopeGlobal, "innodb_adaptive_max_sleep_delay", "150000"},
	{ScopeNone, "large_pages", "OFF"},
	{ScopeGlobal | ScopeSession, "session_track_system_variables", ""},
	{ScopeGlobal, "innodb_change_buffer_max_size", "25"},
	{ScopeGlobal, "log_bin_trust_function_creators", "OFF"},
	{ScopeNone, "innodb_write_io_threads", "4"},
	{ScopeGlobal, "mysql_native_password_proxy_users", ""},
	{ScopeGlobal, "read_only", "OFF"},
	{ScopeNone, "large_page_size", "0"},
	{ScopeNone, "table_open_cache_instances", "1"},
	{ScopeGlobal, "innodb_stats_persistent", "ON"},
	{ScopeGlobal | ScopeSession, "session_track_state_change", ""},
	{ScopeNone, "optimizer_switch", "index_merge=on,index_merge_union=on,index_merge_sort_union=on,index_merge_intersection=on,engine_condition_pushdown=on,index_condition_pushdown=on,mrr=on,mrr_cost_based=on,block_nested_loop=on,batched_key_access=off,materialization=on,semijoin=on,loosescan=on,firstmatch=on,subquery_materialization_cost_based=on,use_index_extensions=on"},
	{ScopeGlobal, "delayed_queue_size", "1000"},
	{ScopeNone, "innodb_read_only", "OFF"},
	{ScopeNone, "datetime_format", "%Y-%m-%d %H:%i:%s"},
	{ScopeGlobal, "log_syslog", ""},
	{ScopeNone, "version", "5.6.25"},
	{ScopeGlobal | ScopeSession, "transaction_alloc_block_size", "8192"},
	{ScopeGlobal, "sql_slave_skip_counter", "0"},
	{ScopeNone, "have_openssl", "DISABLED"},
	{ScopeGlobal, "innodb_large_prefix", "OFF"},
	{ScopeNone, "performance_schema_max_cond_classes", "80"},
	{ScopeGlobal, "innodb_io_capacity", "200"},
	{ScopeGlobal, "max_binlog_cache_size", "18446744073709547520"},
	{ScopeGlobal | ScopeSession, "ndb_index_stat_enable", ""},
	{ScopeGlobal, "executed_gtids_compression_period", ""},
	{ScopeNone, "time_format", "%H:%i:%s"},
	{ScopeGlobal | ScopeSession, "old_alter_table", "OFF"},
	{ScopeGlobal | ScopeSession, "long_query_time", "10.000000"},
	{ScopeNone, "innodb_use_native_aio", "OFF"},
	{ScopeGlobal, "log_throttle_queries_not_using_indexes", "0"},
	{ScopeNone, "locked_in_memory", "OFF"},
	{ScopeNone, "innodb_api_enable_mdl", "OFF"},
	{ScopeGlobal, "binlog_cache_size", "32768"},
	{ScopeGlobal, "innodb_compression_pad_pct_max", "50"},
	{ScopeGlobal, "innodb_commit_concurrency", "0"},
	{ScopeNone, "ft_min_word_len", "4"},
	{ScopeGlobal, "enforce_gtid_consistency", "OFF"},
	{ScopeGlobal, "secure_auth", "ON"},
	{ScopeNone, "max_tmp_tables", "32"},
	{ScopeGlobal, "innodb_random_read_ahead", "OFF"},
	{ScopeGlobal | ScopeSession, "unique_checks", "ON"},
	{ScopeGlobal, "internal_tmp_disk_storage_engine", ""},
	{ScopeGlobal | ScopeSession, "myisam_repair_threads", "1"},
	{ScopeGlobal, "ndb_eventbuffer_max_alloc", ""},
	{ScopeGlobal, "innodb_read_ahead_threshold", "56"},
	{ScopeGlobal, "key_cache_block_size", "1024"},
	{ScopeGlobal, "rpl_semi_sync_slave_enabled", ""},
	{ScopeNone, "ndb_recv_thread_cpu_mask", ""},
	{ScopeGlobal, "gtid_purged", ""},
	{ScopeGlobal, "max_binlog_stmt_cache_size", "18446744073709547520"},
	{ScopeGlobal | ScopeSession, "lock_wait_timeout", "31536000"},
	{ScopeGlobal | ScopeSession, "read_buffer_size", "131072"},
	{ScopeNone, "innodb_read_io_threads", "4"},
	{ScopeGlobal | ScopeSession, "max_sp_recursion_depth", "0"},
	{ScopeNone, "ignore_builtin_innodb", "OFF"},
	{ScopeGlobal, "rpl_semi_sync_master_enabled", ""},
	{ScopeGlobal, "slow_query_log_file", "/usr/local/mysql/data/localhost-slow.log"},
	{ScopeGlobal, "innodb_thread_sleep_delay", "10000"},
	{ScopeNone, "license", "GPL"},
	{ScopeGlobal, "innodb_ft_aux_table", ""},
	{ScopeGlobal | ScopeSession, "sql_warnings", "OFF"},
	{ScopeGlobal | ScopeSession, "keep_files_on_create", "OFF"},
	{ScopeGlobal, "slave_preserve_commit_order", ""},
	{ScopeNone, "innodb_data_file_path", "ibdata1:12M:autoextend"},
	{ScopeNone, "performance_schema_setup_actors_size", "100"},
	{ScopeNone, "innodb_additional_mem_pool_size", "8388608"},
	{ScopeNone, "log_error", "/usr/local/mysql/data/localhost.err"},
	{ScopeGlobal, "slave_exec_mode", "STRICT"},
	{ScopeGlobal, "binlog_stmt_cache_size", "32768"},
	{ScopeNone, "relay_log_info_file", "relay-log.info"},
	{ScopeNone, "innodb_ft_total_cache_size", "640000000"},
	{ScopeNone, "performance_schema_max_rwlock_instances", "9102"},
	{ScopeGlobal, "table_open_cache", "2000"},
	{ScopeNone, "log_slave_updates", "OFF"},
	{ScopeNone, "performance_schema_events_stages_history_long_size", "10000"},
	{ScopeGlobal | ScopeSession, "autocommit", "ON"},
	{ScopeSession, "insert_id", ""},
	{ScopeGlobal | ScopeSession, "default_tmp_storage_engine", "InnoDB"},
	{ScopeGlobal | ScopeSession, "optimizer_search_depth", "62"},
	{ScopeGlobal, "max_points_in_geometry", ""},
	{ScopeGlobal, "innodb_stats_sample_pages", "8"},
	{ScopeGlobal | ScopeSession, "profiling_history_size", "15"},
	{ScopeGlobal | ScopeSession, "character_set_database", "latin1"},
	{ScopeNone, "have_symlink", "YES"},
	{ScopeGlobal | ScopeSession, "storage_engine", "InnoDB"},
	{ScopeGlobal | ScopeSession, "sql_log_off", "OFF"},
	{ScopeNone, "explicit_defaults_for_timestamp", "OFF"},
	{ScopeNone, "performance_schema_events_waits_history_size", "10"},
	{ScopeGlobal, "log_syslog_tag", ""},
	{ScopeGlobal | ScopeSession, "tx_read_only", "OFF"},
	{ScopeGlobal, "rpl_semi_sync_master_wait_point", ""},
	{ScopeGlobal, "innodb_undo_log_truncate", ""},
	{ScopeNone, "simplified_binlog_gtid_recovery", "OFF"},
	{ScopeSession, "innodb_create_intrinsic", ""},
	{ScopeGlobal, "gtid_executed_compression_period", ""},
	{ScopeGlobal, "ndb_log_empty_epochs", ""},
	{ScopeGlobal, "max_prepared_stmt_count", "16382"},
	{ScopeNone, "have_geometry", "YES"},
	{ScopeGlobal | ScopeSession, "optimizer_trace_max_mem_size", "16384"},
	{ScopeGlobal | ScopeSession, "net_retry_count", "10"},
	{ScopeSession, "ndb_table_no_logging", ""},
	{ScopeGlobal | ScopeSession, "optimizer_trace_features", "greedy_search=on,range_optimizer=on,dynamic_range=on,repeated_subselect=on"},
	{ScopeGlobal, "innodb_flush_log_at_trx_commit", "1"},
	{ScopeGlobal, "rewriter_enabled", ""},
	{ScopeGlobal, "query_cache_min_res_unit", "4096"},
	{ScopeGlobal | ScopeSession, "updatable_views_with_limit", "YES"},
	{ScopeGlobal | ScopeSession, "optimizer_prune_level", "1"},
	{ScopeGlobal, "slave_sql_verify_checksum", "ON"},
	{ScopeGlobal | ScopeSession, "completion_type", "NO_CHAIN"},
	{ScopeGlobal, "binlog_checksum", "CRC32"},
	{ScopeNone, "report_port", "3306"},
	{ScopeGlobal | ScopeSession, "show_old_temporals", "OFF"},
	{ScopeGlobal, "query_cache_limit", "1048576"},
	{ScopeGlobal, "innodb_buffer_pool_size", "134217728"},
	{ScopeGlobal, "innodb_adaptive_flushing", "ON"},
	{ScopeNone, "datadir", "/usr/local/mysql/data/"},
	{ScopeGlobal | ScopeSession, "wait_timeout", "28800"},
	{ScopeGlobal, "innodb_monitor_enable", ""},
	{ScopeNone, "date_format", "%Y-%m-%d"},
	{ScopeGlobal, "innodb_buffer_pool_filename", "ib_buffer_pool"},
	{ScopeGlobal, "slow_launch_time", "2"},
	{ScopeGlobal, "slave_max_allowed_packet", "1073741824"},
	{ScopeGlobal | ScopeSession, "ndb_use_transactions", ""},
	{ScopeNone, "innodb_purge_threads", "1"},
	{ScopeGlobal, "innodb_concurrency_tickets", "5000"},
	{ScopeGlobal, "innodb_monitor_reset_all", ""},
	{ScopeNone, "performance_schema_users_size", "100"},
	{ScopeGlobal, "ndb_log_updated_only", ""},
	{ScopeNone, "basedir", "/usr/local/mysql"},
	{ScopeGlobal, "innodb_old_blocks_time", "1000"},
	{ScopeGlobal, "innodb_stats_method", "nulls_equal"},
	{ScopeGlobal | ScopeSession, "innodb_lock_wait_timeout", "50"},
	{ScopeGlobal, "local_infile", "ON"},
	{ScopeGlobal | ScopeSession, "myisam_stats_method", "nulls_unequal"},
	{ScopeNone, "version_compile_os", "osx10.8"},
	{ScopeNone, "relay_log_recovery", "OFF"},
	{ScopeNone, "old", "OFF"},
	{ScopeGlobal | ScopeSession, "innodb_table_locks", "ON"},
	{ScopeNone, "performance_schema", "ON"},
	{ScopeNone, "myisam_recover_options", "OFF"},
	{ScopeGlobal | ScopeSession, "net_buffer_length", "16384"},
	{ScopeGlobal, "rpl_semi_sync_master_wait_for_slave_count", ""},
	{ScopeGlobal | ScopeSession, "binlog_row_image", "FULL"},
	{ScopeNone, "innodb_locks_unsafe_for_binlog", "OFF"},
	{ScopeSession, "rbr_exec_mode", ""},
	{ScopeGlobal, "myisam_max_sort_file_size", "9223372036853727232"},
	{ScopeNone, "back_log", "80"},
	{ScopeNone, "lower_case_file_system", "ON"},
	{ScopeGlobal, "rpl_semi_sync_master_wait_no_slave", ""},
	{ScopeGlobal | ScopeSession, "group_concat_max_len", "1024"},
	{ScopeSession, "pseudo_thread_id", ""},
	{ScopeNone, "socket", "/tmp/myssock"},
	{ScopeNone, "have_dynamic_loading", "YES"},
	{ScopeGlobal, "rewriter_verbose", ""},
	{ScopeGlobal, "innodb_undo_logs", "128"},
	{ScopeNone, "performance_schema_max_cond_instances", "3504"},
	{ScopeGlobal, "delayed_insert_limit", "100"},
	{ScopeGlobal, "flush", "OFF"},
	{ScopeGlobal | ScopeSession, "eq_range_index_dive_limit", "10"},
	{ScopeNone, "performance_schema_events_stages_history_size", "10"},
	{ScopeGlobal | ScopeSession, "character_set_connection", "latin1"},
	{ScopeGlobal, "myisam_use_mmap", "OFF"},
	{ScopeGlobal | ScopeSession, "ndb_join_pushdown", ""},
	{ScopeGlobal | ScopeSession, "character_set_server", "latin1"},
	{ScopeGlobal, "validate_password_special_char_count", ""},
	{ScopeNone, "performance_schema_max_thread_instances", "402"},
	{ScopeGlobal, "slave_rows_search_algorithms", "TABLE_SCAN,INDEX_SCAN"},
	{ScopeGlobal | ScopeSession, "ndbinfo_show_hidden", ""},
	{ScopeGlobal | ScopeSession, "net_read_timeout", "30"},
	{ScopeNone, "innodb_page_size", "16384"},
	{ScopeGlobal, "max_allowed_packet", "4194304"},
	{ScopeNone, "innodb_log_file_size", "50331648"},
	{ScopeGlobal, "sync_relay_log_info", "10000"},
	{ScopeGlobal | ScopeSession, "optimizer_trace_limit", "1"},
	{ScopeNone, "innodb_ft_max_token_size", "84"},
	{ScopeGlobal, "validate_password_length", ""},
	{ScopeGlobal, "ndb_log_binlog_index", ""},
	{ScopeGlobal, "validate_password_mixed_case_count", ""},
	{ScopeGlobal, "innodb_api_bk_commit_interval", "5"},
	{ScopeNone, "innodb_undo_directory", "."},
	{ScopeNone, "bind_address", "*"},
	{ScopeGlobal, "innodb_sync_spin_loops", "30"},
	{ScopeGlobal | ScopeSession, "sql_safe_updates", "OFF"},
	{ScopeNone, "tmpdir", "/var/tmp/"},
	{ScopeGlobal, "innodb_thread_concurrency", "0"},
	{ScopeGlobal, "slave_allow_batching", "OFF"},
	{ScopeGlobal, "innodb_buffer_pool_dump_pct", ""},
	{ScopeGlobal | ScopeSession, "lc_time_names", "en_US"},
	{ScopeGlobal | ScopeSession, "max_statement_time", ""},
	{ScopeGlobal | ScopeSession, "end_markers_in_json", "OFF"},
	{ScopeGlobal, "avoid_temporal_upgrade", "OFF"},
	{ScopeGlobal, "key_cache_age_threshold", "300"},
	{ScopeGlobal, "innodb_status_output", "OFF"},
	{ScopeSession, "identity", ""},
	{ScopeGlobal | ScopeSession, "min_examined_row_limit", "0"},
	{ScopeGlobal, "sync_frm", "ON"},
	{ScopeGlobal, "innodb_online_alter_log_max_size", "134217728"},
}

// SetNamesVariables is the system variable names related to set names statements.
var SetNamesVariables = []string{
	"character_set_client",
	"character_set_connection",
	"character_set_results",
}

const (
	// CollationConnection is the name for collation_connection system variable.
	CollationConnection = "collation_connection"
	// CharsetDatabase is the name for charactor_set_database system variable.
	CharsetDatabase = "character_set_database"
	// CollationDatabase is the name for collation_database system variable.
	CollationDatabase = "collation_database"
)

// GlobalVarAccessor is the interface for accessing global scope system and status variables.
type GlobalVarAccessor interface {
	// GetGlobalSysVar gets the global system variable value for name.
	GetGlobalSysVar(ctx context.Context, name string) (string, error)
	// SetGlobalSysVar sets the global system variable name to value.
	SetGlobalSysVar(ctx context.Context, name string, value string) error
}

// globalSysVarAccessorKeyType is a dummy type to avoid naming collision in context.
type globalSysVarAccessorKeyType int

// String defines a Stringer function for debugging and pretty printing.
func (k globalSysVarAccessorKeyType) String() string {
	return "global_sysvar_accessor"
}

const accessorKey globalSysVarAccessorKeyType = 0

// BindGlobalVarAccessor binds global var accessor to context.
func BindGlobalVarAccessor(ctx context.Context, accessor GlobalVarAccessor) {
	ctx.SetValue(accessorKey, accessor)
}

// GetGlobalVarAccessor gets accessor from ctx.
func GetGlobalVarAccessor(ctx context.Context) GlobalVarAccessor {
	v, ok := ctx.Value(accessorKey).(GlobalVarAccessor)
	if !ok {
		panic("Miss global sysvar accessor")
	}
	return v
}
