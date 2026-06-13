package history

import (
	"github.com/aasixh/devgrep/internal/utils"
)

// ResumeCWD validates incremental indexing cwd state before parser replay.
func ResumeCWD(path, home string) string {
	return utils.SanitizeCWD(path, home, home)
}

// SanitizeRecordCWD normalizes a per-command cwd before persistence.
func SanitizeRecordCWD(path, home string) string {
	if norm, ok := utils.NormalizeCWD(path, home); ok && norm != "" {
		return norm
	}
	return ""
}
