package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aasixh/devgrep/internal/utils"
)

const (
	migrationCleanupCorruptHistoryCWD = 2
	migrationForceHistoryCWDReplay    = 3
)

// RepairCorruptHistoryCWD removes invalid shell history cwd data and resets incremental
// history state when repair is required. It returns true when shell history must be fully replayed.
func (s *Store) RepairCorruptHistoryCWD(ctx context.Context, home string) (bool, error) {
	if home == "" {
		home = utils.MustHome()
	}

	replayRequired, err := s.historyCWDRepairNeeded(ctx)
	if err != nil {
		return false, err
	}

	appliedReplay, err := s.hasMigrationVersion(ctx, migrationForceHistoryCWDReplay)
	if err != nil {
		return false, err
	}
	if !appliedReplay {
		replayRequired = true
		if err := s.recordMigrationVersion(ctx, migrationForceHistoryCWDReplay, "force_history_cwd_replay"); err != nil {
			return false, err
		}
	}

	if !replayRequired {
		return false, nil
	}

	if err := s.deleteAllHistoryDocuments(ctx); err != nil {
		return false, err
	}
	if err := s.ResetShellHistorySourceStates(ctx); err != nil {
		return false, err
	}

	appliedCleanup, err := s.hasMigrationVersion(ctx, migrationCleanupCorruptHistoryCWD)
	if err != nil {
		return false, err
	}
	if !appliedCleanup {
		if err := s.recordMigrationVersion(ctx, migrationCleanupCorruptHistoryCWD, "cleanup_corrupt_history_cwd"); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (s *Store) historyCWDRepairNeeded(ctx context.Context) (bool, error) {
	invalidDocs, err := s.countInvalidHistoryDocuments(ctx)
	if err != nil {
		return false, err
	}
	if invalidDocs > 0 {
		return true, nil
	}
	return s.hasInvalidHistorySourceStates(ctx)
}

func (s *Store) countInvalidHistoryDocuments(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT cwd
FROM documents
WHERE source_type = 'history' AND cwd <> ''`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	invalid := 0
	for rows.Next() {
		var cwd string
		if err := rows.Scan(&cwd); err != nil {
			return 0, err
		}
		if utils.InvalidHistoryCWD(cwd) {
			invalid++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return invalid, nil
}

func (s *Store) hasInvalidHistorySourceStates(ctx context.Context) (bool, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT metadata
FROM source_states
WHERE source_name IN ('bash', 'zsh')`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var metadata string
		if err := rows.Scan(&metadata); err != nil {
			return false, err
		}
		var meta map[string]string
		_ = json.Unmarshal([]byte(metadata), &meta)
		if cwd := meta["cwd"]; cwd != "" && utils.InvalidHistoryCWD(cwd) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) deleteAllHistoryDocuments(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE source_type = 'history'`)
	return err
}

// ResetShellHistorySourceStates clears incremental offsets so history files are fully replayed.
func (s *Store) ResetShellHistorySourceStates(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM source_states WHERE source_name IN ('bash', 'zsh')`)
	return err
}

func (s *Store) hasMigrationVersion(ctx context.Context, version int) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) recordMigrationVersion(ctx context.Context, version int, name string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO schema_migrations (version, name, applied_at)
VALUES (?, ?, ?)`, version, name, time.Now().Unix())
	return err
}
