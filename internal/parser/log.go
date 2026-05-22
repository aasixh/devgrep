package parser

import "strings"

const (
	// SeverityDebug identifies debug log lines.
	SeverityDebug = "DEBUG"
	// SeverityInfo identifies informational log lines.
	SeverityInfo = "INFO"
	// SeverityWarn identifies warning log lines.
	SeverityWarn = "WARN"
	// SeverityError identifies error log lines.
	SeverityError = "ERROR"
)

// DetectSeverity returns the first common log severity found in a line.
func DetectSeverity(line string) string {
	upper := strings.ToUpper(line)
	switch {
	case containsSeverity(upper, SeverityError):
		return SeverityError
	case containsSeverity(upper, SeverityWarn) || containsSeverity(upper, "WARNING"):
		return SeverityWarn
	case containsSeverity(upper, SeverityDebug):
		return SeverityDebug
	case containsSeverity(upper, SeverityInfo):
		return SeverityInfo
	default:
		return ""
	}
}

func containsSeverity(line, severity string) bool {
	if strings.Contains(line, "["+severity+"]") ||
		strings.Contains(line, severity+":") ||
		strings.Contains(line, severity+" ") ||
		strings.Contains(line, " "+severity+" ") {
		return true
	}
	return strings.TrimSpace(line) == severity
}

// ErrorGroupKey normalizes an error line into a stable grouping key.
func ErrorGroupKey(line string) string {
	line = strings.ToLower(line)
	var b strings.Builder
	lastSpace := false
	for _, r := range line {
		switch {
		case r >= '0' && r <= '9':
			if !lastSpace {
				b.WriteByte('#')
				lastSpace = false
			}
		case r == '/' || r == '\\':
			b.WriteByte('/')
			lastSpace = false
		case r == ':' || r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
			lastSpace = false
		case r <= ' ':
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		default:
			b.WriteRune(r)
			lastSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}
