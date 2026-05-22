package search

import (
	"fmt"
	"io"
	"time"

	"github.com/devgrep/devgrep/internal/utils"
)

// PrintPlain writes search results in a Unix-friendly, readable block format.
func PrintPlain(w io.Writer, results []Result, now time.Time) {
	for i, result := range results {
		doc := result.Document
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "[%s]\n%s\n\n", doc.SourceType, doc.Content)
		if doc.SourceType == SourceHistory {
			fmt.Fprintf(w, "[last used]\n%s\n\n", utils.HumanDuration(doc.EventTime, now))
			fmt.Fprintf(w, "[directory]\n%s\n\n", utils.RelHome(doc.CWD))
		} else {
			fmt.Fprintf(w, "[source]\n%s", utils.RelHome(doc.Path))
			if doc.Line > 0 {
				fmt.Fprintf(w, ":%d", doc.Line)
			}
			fmt.Fprint(w, "\n\n")
			if doc.Severity != "" {
				fmt.Fprintf(w, "[severity]\n%s\n\n", doc.Severity)
			}
		}
		fmt.Fprintf(w, "[score]\n%d\n", result.Score)
	}
}
