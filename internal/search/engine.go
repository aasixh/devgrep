package search

import (
	"context"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/devgrep/devgrep/internal/config"
	"github.com/devgrep/devgrep/internal/ranking"
	"github.com/devgrep/devgrep/internal/storage"
	"github.com/devgrep/devgrep/internal/utils"
	"github.com/sahilm/fuzzy"
)

// Engine runs local fuzzy search and ranking over SQLite candidates.
type Engine struct {
	Store  *storage.Store
	Config config.Config
	Now    func() time.Time
}

// Query searches indexed developer memory and returns ranked results.
func (e Engine) Query(ctx context.Context, query string, opts Options) ([]Result, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	now := time.Now()
	if e.Now != nil {
		now = e.Now()
	}
	start := time.Now()

	candidateLimit := opts.Limit * 80
	if candidateLimit < 5000 {
		candidateLimit = 5000
	}
	if candidateLimit > 50000 {
		candidateLimit = 50000
	}
	docs, err := e.Store.SearchCandidates(ctx, query, opts.SourceTypes, candidateLimit)
	if err != nil {
		return nil, err
	}

	results := rankDocuments(query, docs, e.Config, now, opts.Limit)
	if opts.Record {
		_ = e.Store.RecordSearch(ctx, query, len(results), time.Since(start))
	}
	return results, nil
}

func rankDocuments(query string, docs []storage.Document, cfg config.Config, now time.Time, limit int) []Result {
	if len(docs) == 0 {
		return nil
	}
	query = strings.TrimSpace(query)
	targets := make([]string, len(docs))
	for i, doc := range docs {
		targets[i] = doc.Content + " " + doc.CWD + " " + doc.Path
	}

	fuzzyMatches := fuzzy.FindNoSort(query, targets)
	byIndex := make(map[int]fuzzy.Match, len(fuzzyMatches))
	for _, match := range fuzzyMatches {
		byIndex[match.Index] = match
	}

	currentDir, _ := os.Getwd()
	tokens := utils.SearchTokens(query)
	normalizedQuery := utils.NormalizeSearchText(query)
	results := make([]Result, 0, len(fuzzyMatches))
	for i, doc := range docs {
		match, hasFuzzy := byIndex[i]
		exact := normalizedQuery != "" && strings.Contains(doc.Normalized, normalizedQuery)
		approx := approximateScore(tokens, doc.Normalized)
		if query != "" && !hasFuzzy && !exact && approx == 0 {
			continue
		}
		fuzzyScore := match.Score
		if fuzzyScore < approx {
			fuzzyScore = approx
		}
		score := ranking.Score(ranking.Input{
			Document:    doc,
			Query:       query,
			QueryTokens: tokens,
			FuzzyScore:  fuzzyScore,
			ExactPhrase: exact,
			CurrentDir:  currentDir,
			Now:         now,
			Weights:     cfg.Ranking,
		})
		results = append(results, Result{
			Document:       doc,
			Score:          score,
			FuzzyScore:     fuzzyScore,
			MatchedIndexes: match.MatchedIndexes,
			ExactPhrase:    exact,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			if results[i].Document.EventTime.Equal(results[j].Document.EventTime) {
				return results[i].Document.Frequency > results[j].Document.Frequency
			}
			return results[i].Document.EventTime.After(results[j].Document.EventTime)
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func approximateScore(tokens []string, target string) int {
	if len(tokens) == 0 {
		return 20
	}
	targetTokens := strings.Fields(target)
	if len(targetTokens) == 0 {
		return 0
	}
	matched := 0
	best := 0
	for _, token := range tokens {
		tokenBest := 0
		for _, targetToken := range targetTokens {
			if strings.Contains(targetToken, token) {
				tokenBest = 42
				break
			}
			dist := boundedLevenshtein(token, targetToken, 2)
			switch dist {
			case 0:
				tokenBest = max(tokenBest, 38)
			case 1:
				tokenBest = max(tokenBest, 28)
			case 2:
				if len(token) > 4 {
					tokenBest = max(tokenBest, 20)
				}
			}
		}
		if tokenBest > 0 {
			matched++
			best += tokenBest
		}
	}
	if matched == 0 {
		return 0
	}
	return best / len(tokens)
}

func boundedLevenshtein(a, b string, maxDistance int) int {
	if a == b {
		return 0
	}
	ar, br := []rune(a), []rune(b)
	if diff := len(ar) - len(br); diff > maxDistance || diff < -maxDistance {
		return maxDistance + 1
	}
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		rowMin := curr[0]
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			curr[j] = min(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
			if curr[j] < rowMin {
				rowMin = curr[j]
			}
		}
		if rowMin > maxDistance {
			return maxDistance + 1
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

func min(values ...int) int {
	m := values[0]
	for _, value := range values[1:] {
		if value < m {
			m = value
		}
	}
	return m
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
