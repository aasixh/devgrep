package storage

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_type TEXT NOT NULL,
		source_name TEXT NOT NULL,
		content TEXT NOT NULL,
		normalized TEXT NOT NULL,
		cwd TEXT NOT NULL DEFAULT '',
		path TEXT NOT NULL DEFAULT '',
		line INTEGER NOT NULL DEFAULT 0,
		severity TEXT NOT NULL DEFAULT '',
		event_time INTEGER NOT NULL,
		first_seen_at INTEGER NOT NULL,
		last_seen_at INTEGER NOT NULL,
		frequency INTEGER NOT NULL DEFAULT 1,
		hash TEXT NOT NULL UNIQUE,
		metadata TEXT NOT NULL DEFAULT '{}'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_documents_source ON documents(source_type, source_name)`,
	`CREATE INDEX IF NOT EXISTS idx_documents_event_time ON documents(event_time DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_documents_frequency ON documents(frequency DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_documents_normalized ON documents(normalized)`,
	`CREATE TABLE IF NOT EXISTS source_states (
		source_name TEXT NOT NULL,
		path TEXT NOT NULL,
		size INTEGER NOT NULL DEFAULT 0,
		mod_time INTEGER NOT NULL DEFAULT 0,
		line_offset INTEGER NOT NULL DEFAULT 0,
		metadata TEXT NOT NULL DEFAULT '{}',
		last_indexed_at INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (source_name, path)
	)`,
	`CREATE TABLE IF NOT EXISTS search_stats (
		query TEXT PRIMARY KEY,
		count INTEGER NOT NULL DEFAULT 0,
		last_searched_at INTEGER NOT NULL,
		total_duration_ms INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE TABLE IF NOT EXISTS index_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_name TEXT NOT NULL,
		indexed_count INTEGER NOT NULL DEFAULT 0,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		error TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS watched_paths (
		path TEXT PRIMARY KEY,
		added_at INTEGER NOT NULL,
		last_seen_at INTEGER NOT NULL
	)`,
	`INSERT OR IGNORE INTO schema_migrations (version, name, applied_at)
	 VALUES (1, 'initial', strftime('%s', 'now'))`,
}
