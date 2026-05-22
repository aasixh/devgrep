package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devgrep/devgrep/internal/utils"
	"gopkg.in/yaml.v3"
)

// RankingConfig controls how result scores are blended.
type RankingConfig struct {
	Fuzzy              float64 `yaml:"fuzzy"`
	Recency            float64 `yaml:"recency"`
	Frequency          float64 `yaml:"frequency"`
	Exact              float64 `yaml:"exact"`
	CommandLength      float64 `yaml:"command_length"`
	DirectoryRelevance float64 `yaml:"directory_relevance"`
}

// ThemeConfig controls terminal colors.
type ThemeConfig struct {
	Name    string `yaml:"name"`
	Accent  string `yaml:"accent"`
	Muted   string `yaml:"muted"`
	Error   string `yaml:"error"`
	Warning string `yaml:"warning"`
	Success string `yaml:"success"`
}

// HistoryConfig controls shell history indexing.
type HistoryConfig struct {
	Limit int `yaml:"limit"`
}

// LogsConfig controls plaintext log indexing and tailing.
type LogsConfig struct {
	MaxFileSizeMB int      `yaml:"max_file_size_mb"`
	Extensions    []string `yaml:"extensions"`
}

// IndexConfig controls safe file discovery and watch behavior.
type IndexConfig struct {
	MaxFiles      int  `yaml:"max_files"`
	MaxFileSizeMB int  `yaml:"max_file_size_mb"`
	AutoWatch     bool `yaml:"auto_watch"`
}

// Config is the devgrep runtime configuration.
type Config struct {
	ConfigPath         string        `yaml:"-"`
	DatabasePath       string        `yaml:"database_path"`
	IndexedPaths       []string      `yaml:"indexed_paths"`
	IgnoredDirectories []string      `yaml:"ignored_directories"`
	Ranking            RankingConfig `yaml:"ranking"`
	TUI                ThemeConfig   `yaml:"tui"`
	History            HistoryConfig `yaml:"history"`
	Logs               LogsConfig    `yaml:"logs"`
	Indexing           IndexConfig   `yaml:"indexing"`
}

// Default returns a zero-configuration production default.
func Default() Config {
	home := utils.MustHome()
	return Config{
		ConfigPath:   filepath.Join(home, ".config", "devgrep", "config.yaml"),
		DatabasePath: filepath.Join(home, ".local", "share", "devgrep", "devgrep.db"),
		IndexedPaths: []string{
			".",
			"~/notes",
			"~/Documents",
		},
		IgnoredDirectories: utils.DefaultIgnoredDirectories(),
		Ranking: RankingConfig{
			Fuzzy:              0.38,
			Recency:            0.26,
			Frequency:          0.10,
			Exact:              0.16,
			CommandLength:      0.02,
			DirectoryRelevance: 0.08,
		},
		TUI: ThemeConfig{
			Name:    "devgrep",
			Accent:  "#7DD3FC",
			Muted:   "#6B7280",
			Error:   "#F87171",
			Warning: "#FBBF24",
			Success: "#34D399",
		},
		History: HistoryConfig{Limit: 200000},
		Logs:    HistoryConfigToLogs(),
		Indexing: IndexConfig{
			MaxFiles:      100000,
			MaxFileSizeMB: 32,
			AutoWatch:     true,
		},
	}
}

// HistoryConfigToLogs returns the default log indexing policy.
func HistoryConfigToLogs() LogsConfig {
	return LogsConfig{
		MaxFileSizeMB: 32,
		Extensions:    []string{".log"},
	}
}

// Load reads config from path, returning defaults when the file does not exist.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		expanded, err := utils.ExpandHome(path)
		if err != nil {
			return cfg, err
		}
		cfg.ConfigPath = expanded
	}

	data, err := os.ReadFile(cfg.ConfigPath)
	if errors.Is(err, os.ErrNotExist) {
		return normalize(cfg)
	}
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	cfg.ConfigPath = firstNonEmpty(pathOrDefault(path), cfg.ConfigPath)
	return normalize(cfg)
}

// EnsureDefaultFile writes a default config file if one does not exist.
func EnsureDefaultFile(cfg Config) error {
	if _, err := os.Stat(cfg.ConfigPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := utils.EnsureDir(filepath.Dir(cfg.ConfigPath)); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.ConfigPath, data, 0o644)
}

func normalize(cfg Config) (Config, error) {
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = Default().DatabasePath
	}
	dbPath, err := utils.ExpandHome(cfg.DatabasePath)
	if err != nil {
		return cfg, err
	}
	cfg.DatabasePath = dbPath
	if cfg.IndexedPaths == nil {
		cfg.IndexedPaths = Default().IndexedPaths
	}
	for i, path := range cfg.IndexedPaths {
		expanded, err := utils.ExpandHome(path)
		if err != nil {
			return cfg, err
		}
		cfg.IndexedPaths[i] = expanded
	}
	if cfg.IgnoredDirectories == nil {
		cfg.IgnoredDirectories = Default().IgnoredDirectories
	}
	if cfg.Ranking.Fuzzy == 0 {
		cfg.Ranking = Default().Ranking
	}
	if cfg.TUI.Name == "" {
		cfg.TUI = Default().TUI
	}
	if cfg.History.Limit <= 0 {
		cfg.History = Default().History
	}
	if cfg.Logs.MaxFileSizeMB <= 0 {
		cfg.Logs = Default().Logs
	}
	if cfg.Logs.Extensions == nil {
		cfg.Logs.Extensions = Default().Logs.Extensions
	}
	if cfg.Indexing.MaxFiles <= 0 {
		cfg.Indexing.MaxFiles = Default().Indexing.MaxFiles
	}
	if cfg.Indexing.MaxFileSizeMB <= 0 {
		cfg.Indexing.MaxFileSizeMB = Default().Indexing.MaxFileSizeMB
	}
	return cfg, nil
}

// Validate returns human-readable configuration issues.
func Validate(cfg Config) []string {
	var issues []string
	if cfg.DatabasePath == "" {
		issues = append(issues, "database_path is empty")
	}
	if cfg.History.Limit <= 0 {
		issues = append(issues, "history.limit must be greater than zero")
	}
	if cfg.Logs.MaxFileSizeMB <= 0 {
		issues = append(issues, "logs.max_file_size_mb must be greater than zero")
	}
	if len(cfg.Logs.Extensions) == 0 {
		issues = append(issues, "logs.extensions must include at least one extension")
	}
	if cfg.Indexing.MaxFiles <= 0 {
		issues = append(issues, "indexing.max_files must be greater than zero")
	}
	if cfg.Indexing.MaxFileSizeMB <= 0 {
		issues = append(issues, "indexing.max_file_size_mb must be greater than zero")
	}
	if cfg.Ranking.Fuzzy < 0 || cfg.Ranking.Recency < 0 || cfg.Ranking.Frequency < 0 || cfg.Ranking.Exact < 0 || cfg.Ranking.CommandLength < 0 || cfg.Ranking.DirectoryRelevance < 0 {
		issues = append(issues, "ranking weights must be zero or positive")
	}
	for _, path := range cfg.IndexedPaths {
		if path == "" {
			issues = append(issues, "indexed_paths contains an empty path")
			continue
		}
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			issues = append(issues, "indexed path missing: "+utils.RelHome(path))
			continue
		}
		if err != nil {
			issues = append(issues, "indexed path unreadable: "+utils.RelHome(path))
			continue
		}
		if info.IsDir() {
			if file, err := os.Open(path); err != nil {
				issues = append(issues, "indexed directory unreadable: "+utils.RelHome(path))
			} else {
				_ = file.Close()
			}
		}
	}
	return issues
}

func pathOrDefault(path string) string {
	if path == "" {
		return Default().ConfigPath
	}
	expanded, err := utils.ExpandHome(path)
	if err != nil {
		return path
	}
	return expanded
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
