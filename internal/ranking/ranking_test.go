package ranking

import (
	"testing"
	"time"

	"github.com/devgrep/devgrep/internal/config"
	"github.com/devgrep/devgrep/internal/storage"
)

func TestScorePrefersRecentExactFrequentCommands(t *testing.T) {
	now := time.Unix(1700000000, 0)
	high := Score(Input{
		Document: storage.Document{
			Content:    "docker compose up -d postgres",
			Normalized: "docker compose up -d postgres",
			EventTime:  now.Add(-time.Hour),
			Frequency:  10,
			CWD:        "/work/api",
		},
		Query:       "docker postgres",
		QueryTokens: []string{"docker", "postgres"},
		FuzzyScore:  110,
		CurrentDir:  "/work/api",
		Now:         now,
		Weights:     config.Default().Ranking,
	})
	low := Score(Input{
		Document: storage.Document{
			Content:    "kubectl get pods",
			Normalized: "kubectl get pods",
			EventTime:  now.Add(-200 * 24 * time.Hour),
			Frequency:  1,
			CWD:        "/elsewhere",
		},
		Query:       "docker postgres",
		QueryTokens: []string{"docker", "postgres"},
		FuzzyScore:  10,
		CurrentDir:  "/work/api",
		Now:         now,
		Weights:     config.Default().Ranking,
	})
	if high <= low {
		t.Fatalf("high score %d <= low score %d", high, low)
	}
}
