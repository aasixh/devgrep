package ranking

import (
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/devgrep/devgrep/internal/config"
	"github.com/devgrep/devgrep/internal/storage"
	"github.com/devgrep/devgrep/internal/utils"
)

// Input contains the raw signals used by the scoring engine.
type Input struct {
	Document    storage.Document
	Query       string
	QueryTokens []string
	FuzzyScore  int
	ExactPhrase bool
	CurrentDir  string
	Now         time.Time
	Weights     config.RankingConfig
}

// Score blends fuzzy match quality, recency, frequency, exactness, length, and directory relevance.
func Score(input Input) int {
	if input.Now.IsZero() {
		input.Now = time.Now()
	}
	weights := input.Weights
	if weights.Fuzzy == 0 {
		weights = config.Default().Ranking
	}

	fuzzy := normalizeFuzzy(input.FuzzyScore)
	recency := recencyScore(input.Document.EventTime, input.Now)
	frequency := frequencyScore(input.Document.Frequency)
	exact := exactScore(input)
	length := lengthScore(input.Document.Content)
	dir := directoryScore(input.Document.CWD, input.CurrentDir, input.QueryTokens)

	sum := weights.Fuzzy + weights.Recency + weights.Frequency + weights.Exact + weights.CommandLength + weights.DirectoryRelevance
	if sum <= 0 {
		sum = 1
	}
	score := fuzzy*weights.Fuzzy +
		recency*weights.Recency +
		frequency*weights.Frequency +
		exact*weights.Exact +
		length*weights.CommandLength +
		dir*weights.DirectoryRelevance
	return clampInt(int(math.Round(score/sum)), 0, 100)
}

func normalizeFuzzy(raw int) float64 {
	if raw <= 0 {
		return 0
	}
	if raw >= 120 {
		return 100
	}
	return float64(raw) / 120 * 100
}

func recencyScore(t time.Time, now time.Time) float64 {
	if t.IsZero() {
		return 35
	}
	days := now.Sub(t).Hours() / 24
	if days < 0 {
		days = 0
	}
	return 100 / (1 + days/14)
}

func frequencyScore(frequency int) float64 {
	if frequency <= 1 {
		return 15
	}
	return math.Min(100, math.Log1p(float64(frequency))*28)
}

func exactScore(input Input) float64 {
	if strings.TrimSpace(input.Query) == "" {
		return 50
	}
	if input.ExactPhrase {
		return 100
	}
	if len(input.QueryTokens) == 0 {
		return 0
	}
	normalized := input.Document.Normalized
	matched := 0
	for _, token := range input.QueryTokens {
		if strings.Contains(normalized, token) {
			matched++
		}
	}
	return float64(matched) / float64(len(input.QueryTokens)) * 82
}

func lengthScore(command string) float64 {
	runes := len([]rune(command))
	switch {
	case runes == 0:
		return 0
	case runes <= 80:
		return 100
	case runes <= 160:
		return 82
	case runes <= 320:
		return 62
	default:
		return 42
	}
}

func directoryScore(cwd, currentDir string, tokens []string) float64 {
	if cwd == "" {
		return 35
	}
	cwd = filepath.Clean(cwd)
	if currentDir != "" {
		currentDir = filepath.Clean(currentDir)
		if cwd == currentDir {
			return 100
		}
		if rel, err := filepath.Rel(currentDir, cwd); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return 92
		}
		if rel, err := filepath.Rel(cwd, currentDir); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return 88
		}
	}
	lower := utils.NormalizeSearchText(cwd)
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			return 70
		}
	}
	return 35
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
