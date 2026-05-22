package parser

import (
	"bufio"
	"io"
	"strings"
)

// Fragment is a searchable plaintext fragment from a note.
type Fragment struct {
	Line    int
	Content string
}

// ParseMarkdownFragments groups markdown into headings and compact paragraphs.
func ParseMarkdownFragments(r io.Reader) ([]Fragment, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var fragments []Fragment
	var buf []string
	startLine := 1
	lineNo := 0

	flush := func() {
		if len(buf) == 0 {
			return
		}
		content := strings.TrimSpace(strings.Join(buf, " "))
		if content != "" {
			fragments = append(fragments, Fragment{Line: startLine, Content: content})
		}
		buf = nil
	}

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			startLine = lineNo
			buf = append(buf, strings.TrimSpace(strings.TrimLeft(line, "#")))
			flush()
			continue
		}
		if len(buf) == 0 {
			startLine = lineNo
		}
		buf = append(buf, line)
		if len(buf) >= 8 {
			flush()
		}
	}
	flush()
	return fragments, scanner.Err()
}
