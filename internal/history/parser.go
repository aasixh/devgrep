package history

import (
	"bufio"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devgrep/devgrep/internal/utils"
)

// Record is one parsed shell history command.
type Record struct {
	Command   string
	Timestamp time.Time
	Shell     string
	CWD       string
}

// ParseResult contains parsed history and the inferred final directory.
type ParseResult struct {
	Records []Record
	EndCWD  string
	Lines   int
}

// ParseBash parses bash history, including HISTTIMEFORMAT timestamp lines.
func ParseBash(r io.Reader, home, startCWD string) (ParseResult, error) {
	return parseLines(r, "bash", home, startCWD, parseBashLine)
}

// ParseZsh parses zsh extended history and plain zsh history.
func ParseZsh(r io.Reader, home, startCWD string) (ParseResult, error) {
	return parseLines(r, "zsh", home, startCWD, parseZshLine)
}

type parsedLine struct {
	command   string
	timestamp time.Time
	isTime    bool
}

func parseLines(r io.Reader, shell, home, startCWD string, parse func(string) parsedLine) (ParseResult, error) {
	if home == "" {
		home = utils.MustHome()
	}
	current := sanitizeParserCWD(startCWD, home)
	previous := current
	pendingTimestamp := time.Time{}
	result := ParseResult{EndCWD: current}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		result.Lines++
		line := strings.TrimRight(scanner.Text(), "\r")
		parsed := parse(line)
		if parsed.isTime {
			pendingTimestamp = parsed.timestamp
			continue
		}
		command := strings.TrimSpace(parsed.command)
		if command == "" {
			continue
		}
		ts := parsed.timestamp
		if ts.IsZero() {
			ts = pendingTimestamp
		}
		pendingTimestamp = time.Time{}
		result.Records = append(result.Records, Record{
			Command:   command,
			Timestamp: ts,
			Shell:     shell,
			CWD:       current,
		})
		current, previous = applyDirectoryCommand(command, current, previous, home)
		result.EndCWD = current
	}
	return result, scanner.Err()
}

func parseBashLine(line string) parsedLine {
	if strings.HasPrefix(line, "#") && len(line) > 1 {
		if unix, err := strconv.ParseInt(line[1:], 10, 64); err == nil {
			return parsedLine{timestamp: time.Unix(unix, 0), isTime: true}
		}
	}
	return parsedLine{command: line}
}

func parseZshLine(line string) parsedLine {
	if !strings.HasPrefix(line, ": ") {
		return parsedLine{command: line}
	}
	semi := strings.Index(line, ";")
	if semi <= 2 {
		return parsedLine{command: line}
	}
	header := strings.TrimPrefix(line[:semi], ": ")
	fields := strings.Split(header, ":")
	if len(fields) == 0 {
		return parsedLine{command: line}
	}
	unix, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 64)
	if err != nil {
		return parsedLine{command: line}
	}
	rest := strings.TrimSpace(line[semi+1:])
	command := rest
	// Extended zsh history may include an absolute cwd before the command:
	// : <unix>:<elapsed>;<cwd>;<command>
	if pathPart, cmdPart, ok := strings.Cut(rest, ";"); ok {
		pathPart = strings.TrimSpace(pathPart)
		cmdPart = strings.TrimSpace(cmdPart)
		if filepath.IsAbs(pathPart) && cmdPart != "" {
			command = cmdPart
		}
	}
	return parsedLine{command: command, timestamp: time.Unix(unix, 0)}
}

func extractCDTarget(command string) (string, bool) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", false
	}
	name, rest, ok := strings.Cut(command, " ")
	if !ok {
		rest = ""
	}
	switch name {
	case "cd", "chdir", "pushd":
	default:
		return "", false
	}
	return parseShellPathToken(strings.TrimSpace(rest)), true
}

func parseShellPathToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, sep := range []string{" && ", " || ", " ; ", "&", "|"} {
		if idx := strings.Index(s, sep); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	if len(s) >= 2 {
		switch s[0] {
		case '"', '\'':
			quote := s[0]
			if end := strings.IndexByte(s[1:], quote); end >= 0 {
				return s[1 : end+1]
			}
		}
	}
	return strings.Trim(s, `"'`)
}

func resolveCDTarget(target, current, home string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		if norm, ok := utils.NormalizeCWD(home, home); ok {
			return norm, true
		}
		return "", false
	}
	expanded, err := utils.ExpandPathWithHome(target, home)
	if err != nil {
		return "", false
	}
	if expanded == "" {
		expanded = target
	}
	if !filepath.IsAbs(expanded) {
		if current == "" {
			current = home
		}
		expanded = filepath.Join(current, expanded)
	}
	clean := filepath.Clean(expanded)
	if utils.MalformedCWDPath(clean) {
		return "", false
	}
	if norm, ok := utils.NormalizeCWD(clean, home); ok {
		return norm, true
	}
	return "", false
}

func applyDirectoryCommand(command, current, previous, home string) (string, string) {
	target, ok := extractCDTarget(command)
	if !ok {
		return current, previous
	}
	if target == "-" {
		if previous == "" || utils.MalformedCWDPath(previous) {
			return current, previous
		}
		return previous, current
	}
	next, ok := resolveCDTarget(target, current, home)
	if !ok {
		return current, previous
	}
	return next, current
}

func sanitizeParserCWD(path, home string) string {
	return utils.SanitizeCWD(path, home, home)
}
