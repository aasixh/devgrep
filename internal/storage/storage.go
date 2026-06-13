package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aasixh/devgrep/internal/utils"
	_ "modernc.org/sqlite"
)

// Store owns the SQLite connection and all database access.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens and migrates a SQLite database.
func Open(ctx context.Context, path string) (*Store, error) {
	expanded, err := utils.ExpandHome(path)
	if err != nil {
		return nil, err
	}
	if err := utils.EnsureDir(filepath.Dir(expanded)); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", expanded)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &Store{db: db, path: expanded}
	if err := store.configure(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) configure(ctx context.Context) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA temp_store=MEMORY",
	}
	for _, stmt := range pragmas {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the SQLite connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Path returns the database file path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Migrate applies schema migrations.
func (s *Store) Migrate(ctx context.Context) error {
	for _, migration := range migrations {
		if _, err := s.db.ExecContext(ctx, migration); err != nil {
			return err
		}
	}
	_, err := s.RepairCorruptHistoryCWD(ctx, utils.MustHome())
	return err
}

// Document is an indexed command, log line, or note fragment.
type Document struct {
	ID         int64
	SourceType string
	SourceName string
	Content    string
	Normalized string
	CWD        string
	Path       string
	Line       int
	Severity   string
	EventTime  time.Time
	FirstSeen  time.Time
	LastSeen   time.Time
	Frequency  int
	Hash       string
	Metadata   map[string]string
}

// SourceState stores incremental indexing offsets for a source file.
type SourceState struct {
	SourceName    string
	Path          string
	Size          int64
	ModTime       time.Time
	LineOffset    int
	Metadata      map[string]string
	LastIndexedAt time.Time
}

// IndexRun stores one index execution summary.
type IndexRun struct {
	SourceName string
	Indexed    int
	Duration   time.Duration
	Error      string
}

// Stats summarizes the local devgrep database.
type Stats struct {
	TotalDocuments int64
	DBSizeBytes    int64
	BySource       map[string]int64
	TopSearches    []SearchStat
	LastRuns       []IndexRun
}

// SearchStat records a searched query and count.
type SearchStat struct {
	Query string
	Count int64
}

// SourceLocation is one indexed source path grouped for display.
type SourceLocation struct {
	Type string
	Path string
}

// UpsertDocuments writes documents using a prepared statement.
func (s *Store) UpsertDocuments(ctx context.Context, docs []Document) (int, error) {
	if len(docs) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO documents (
	source_type, source_name, content, normalized, cwd, path, line, severity,
	event_time, first_seen_at, last_seen_at, frequency, hash, metadata
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(hash) DO UPDATE SET
	last_seen_at=excluded.last_seen_at,
	event_time=max(documents.event_time, excluded.event_time),
	frequency=documents.frequency + excluded.frequency,
	cwd=coalesce(nullif(excluded.cwd, ''), documents.cwd),
	path=coalesce(nullif(excluded.path, ''), documents.path),
	line=excluded.line,
	severity=coalesce(nullif(excluded.severity, ''), documents.severity),
	metadata=excluded.metadata
`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	now := time.Now()
	written := 0
	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		doc = normalizeDocument(doc, now)
		metadata, err := json.Marshal(doc.Metadata)
		if err != nil {
			return written, err
		}
		if _, err := stmt.ExecContext(ctx,
			doc.SourceType,
			doc.SourceName,
			doc.Content,
			doc.Normalized,
			doc.CWD,
			doc.Path,
			doc.Line,
			doc.Severity,
			doc.EventTime.Unix(),
			doc.FirstSeen.Unix(),
			doc.LastSeen.Unix(),
			doc.Frequency,
			doc.Hash,
			string(metadata),
		); err != nil {
			return written, err
		}
		written++
	}
	return written, tx.Commit()
}

func normalizeDocument(doc Document, now time.Time) Document {
	if doc.Normalized == "" {
		doc.Normalized = utils.NormalizeSearchText(doc.Content + " " + doc.CWD + " " + doc.Path)
	}
	if doc.EventTime.IsZero() {
		doc.EventTime = now
	}
	if doc.FirstSeen.IsZero() {
		doc.FirstSeen = now
	}
	if doc.LastSeen.IsZero() {
		doc.LastSeen = now
	}
	if doc.Frequency <= 0 {
		doc.Frequency = 1
	}
	if doc.Metadata == nil {
		doc.Metadata = map[string]string{}
	}
	if doc.Hash == "" {
		doc.Hash = utils.HashString(doc.SourceType, doc.SourceName, doc.Content, doc.CWD, doc.Path, fmt.Sprint(doc.Line))
	}
	return doc
}

// SearchCandidates returns recent and token-matching candidates for ranking.
func (s *Store) SearchCandidates(ctx context.Context, query string, sourceTypes []string, limit int) ([]Document, error) {
	if limit <= 0 {
		limit = 5000
	}
	where := []string{"1=1"}
	args := []any{}
	if len(sourceTypes) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(sourceTypes)), ",")
		where = append(where, "source_type IN ("+placeholders+")")
		for _, source := range sourceTypes {
			args = append(args, source)
		}
	}
	tokens := utils.SearchTokens(query)
	if len(tokens) > 0 {
		or := make([]string, 0, len(tokens))
		for _, token := range tokens {
			or = append(or, "normalized LIKE ? ESCAPE '\\'")
			args = append(args, "%"+escapeLike(token)+"%")
		}
		where = append(where, "("+strings.Join(or, " OR ")+")")
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, source_type, source_name, content, normalized, cwd, path, line, severity,
       event_time, first_seen_at, last_seen_at, frequency, hash, metadata
FROM documents
WHERE `+strings.Join(where, " AND ")+`
ORDER BY event_time DESC, frequency DESC
LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	docs, err := scanDocuments(rows)
	if err != nil {
		return nil, err
	}
	if len(tokens) > 0 && len(docs) > 0 {
		return docs, nil
	}
	if len(docs) >= limit || len(tokens) == 0 {
		return docs, nil
	}

	seen := make(map[int64]struct{}, len(docs))
	for _, doc := range docs {
		seen[doc.ID] = struct{}{}
	}
	recent, err := s.recentCandidates(ctx, sourceTypes, limit)
	if err != nil {
		return docs, nil
	}
	for _, doc := range recent {
		if _, ok := seen[doc.ID]; ok {
			continue
		}
		docs = append(docs, doc)
		if len(docs) >= limit {
			break
		}
	}
	return docs, nil
}

func (s *Store) recentCandidates(ctx context.Context, sourceTypes []string, limit int) ([]Document, error) {
	where := []string{"1=1"}
	args := []any{}
	if len(sourceTypes) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(sourceTypes)), ",")
		where = append(where, "source_type IN ("+placeholders+")")
		for _, source := range sourceTypes {
			args = append(args, source)
		}
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, source_type, source_name, content, normalized, cwd, path, line, severity,
       event_time, first_seen_at, last_seen_at, frequency, hash, metadata
FROM documents
WHERE `+strings.Join(where, " AND ")+`
ORDER BY event_time DESC, frequency DESC
LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocuments(rows)
}

// DeleteBySourcePath removes documents for a source/path pair before full reindexing.
func (s *Store) DeleteBySourcePath(ctx context.Context, sourceType, path string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE source_type = ? AND path = ?`, sourceType, path)
	return err
}

// DeleteByPath removes all documents indexed from a file path.
func (s *Store) DeleteByPath(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE path = ?`, path)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM source_states WHERE path = ?`, path)
	return err
}

// DocumentCount returns the number of indexed documents.
func (s *Store) DocumentCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM documents`).Scan(&count)
	return count, err
}

// SourceLocations returns deduplicated indexed source locations.
func (s *Store) SourceLocations(ctx context.Context) ([]SourceLocation, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT source_type, path
FROM documents
WHERE path <> ''
GROUP BY source_type, path
ORDER BY source_type, path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SourceLocation
	for rows.Next() {
		var item SourceLocation
		if err := rows.Scan(&item.Type, &item.Path); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// SaveWatchedPath persists a path for automatic watch restoration.
func (s *Store) SaveWatchedPath(ctx context.Context, path string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO watched_paths (path, added_at, last_seen_at)
VALUES (?, ?, ?)
ON CONFLICT(path) DO UPDATE SET last_seen_at=excluded.last_seen_at`, path, now, now)
	return err
}

// WatchedPaths returns persisted watched directories.
func (s *Store) WatchedPaths(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path FROM watched_paths ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

// RemoveWatchedPath deletes a persisted watch path.
func (s *Store) RemoveWatchedPath(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM watched_paths WHERE path = ?`, path)
	return err
}

func scanDocuments(rows *sql.Rows) ([]Document, error) {
	var docs []Document
	for rows.Next() {
		var doc Document
		var eventUnix, firstUnix, lastUnix int64
		var metadata string
		if err := rows.Scan(&doc.ID, &doc.SourceType, &doc.SourceName, &doc.Content, &doc.Normalized, &doc.CWD, &doc.Path, &doc.Line, &doc.Severity, &eventUnix, &firstUnix, &lastUnix, &doc.Frequency, &doc.Hash, &metadata); err != nil {
			return nil, err
		}
		doc.EventTime = time.Unix(eventUnix, 0)
		doc.FirstSeen = time.Unix(firstUnix, 0)
		doc.LastSeen = time.Unix(lastUnix, 0)
		_ = json.Unmarshal([]byte(metadata), &doc.Metadata)
		if doc.Metadata == nil {
			doc.Metadata = map[string]string{}
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// GetSourceState loads incremental state for a source path.
func (s *Store) GetSourceState(ctx context.Context, sourceName, path string) (SourceState, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT source_name, path, size, mod_time, line_offset, metadata, last_indexed_at
FROM source_states
WHERE source_name = ? AND path = ?`, sourceName, path)
	var state SourceState
	var modUnix, indexedUnix int64
	var metadata string
	if err := row.Scan(&state.SourceName, &state.Path, &state.Size, &modUnix, &state.LineOffset, &metadata, &indexedUnix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SourceState{}, false, nil
		}
		return SourceState{}, false, err
	}
	state.ModTime = time.Unix(modUnix, 0)
	state.LastIndexedAt = time.Unix(indexedUnix, 0)
	_ = json.Unmarshal([]byte(metadata), &state.Metadata)
	if state.Metadata == nil {
		state.Metadata = map[string]string{}
	}
	return state, true, nil
}

// SaveSourceState writes incremental state for a source path.
func (s *Store) SaveSourceState(ctx context.Context, state SourceState) error {
	if state.Metadata == nil {
		state.Metadata = map[string]string{}
	}
	if state.LastIndexedAt.IsZero() {
		state.LastIndexedAt = time.Now()
	}
	data, err := json.Marshal(state.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO source_states (source_name, path, size, mod_time, line_offset, metadata, last_indexed_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source_name, path) DO UPDATE SET
	size=excluded.size,
	mod_time=excluded.mod_time,
	line_offset=excluded.line_offset,
	metadata=excluded.metadata,
	last_indexed_at=excluded.last_indexed_at`,
		state.SourceName,
		state.Path,
		state.Size,
		state.ModTime.Unix(),
		state.LineOffset,
		string(data),
		state.LastIndexedAt.Unix(),
	)
	return err
}

// RecordIndexRun stores one index execution summary.
func (s *Store) RecordIndexRun(ctx context.Context, run IndexRun) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO index_runs (source_name, indexed_count, duration_ms, error, created_at)
VALUES (?, ?, ?, ?, ?)`,
		run.SourceName,
		run.Indexed,
		run.Duration.Milliseconds(),
		run.Error,
		time.Now().Unix(),
	)
	return err
}

// RecordSearch stores query statistics.
func (s *Store) RecordSearch(ctx context.Context, query string, resultCount int, duration time.Duration) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO search_stats (query, count, last_searched_at, total_duration_ms)
VALUES (?, 1, ?, ?)
ON CONFLICT(query) DO UPDATE SET
	count=count + 1,
	last_searched_at=excluded.last_searched_at,
	total_duration_ms=total_duration_ms + excluded.total_duration_ms`,
		query,
		time.Now().Unix(),
		duration.Milliseconds(),
	)
	return err
}

// Stats returns database counters and recent activity.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	stats := Stats{BySource: map[string]int64{}}
	if info, err := os.Stat(s.path); err == nil {
		stats.DBSizeBytes = info.Size()
	}
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM documents`).Scan(&stats.TotalDocuments); err != nil {
		return stats, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT source_type, count(*) FROM documents GROUP BY source_type ORDER BY 2 DESC`)
	if err != nil {
		return stats, err
	}
	for rows.Next() {
		var source string
		var count int64
		if err := rows.Scan(&source, &count); err != nil {
			rows.Close()
			return stats, err
		}
		stats.BySource[source] = count
	}
	rows.Close()

	searchRows, err := s.db.QueryContext(ctx, `SELECT query, count FROM search_stats ORDER BY count DESC, last_searched_at DESC LIMIT 10`)
	if err != nil {
		return stats, err
	}
	for searchRows.Next() {
		var item SearchStat
		if err := searchRows.Scan(&item.Query, &item.Count); err != nil {
			searchRows.Close()
			return stats, err
		}
		stats.TopSearches = append(stats.TopSearches, item)
	}
	searchRows.Close()

	runRows, err := s.db.QueryContext(ctx, `SELECT source_name, indexed_count, duration_ms, error FROM index_runs ORDER BY created_at DESC LIMIT 8`)
	if err != nil {
		return stats, err
	}
	for runRows.Next() {
		var run IndexRun
		var ms int64
		if err := runRows.Scan(&run.SourceName, &run.Indexed, &ms, &run.Error); err != nil {
			runRows.Close()
			return stats, err
		}
		run.Duration = time.Duration(ms) * time.Millisecond
		stats.LastRuns = append(stats.LastRuns, run)
	}
	runRows.Close()

	return stats, nil
}

// IntegrityCheck runs SQLite's integrity check.
func (s *Store) IntegrityCheck(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("sqlite integrity_check: %s", result)
	}
	return nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
