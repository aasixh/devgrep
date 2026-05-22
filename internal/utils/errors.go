package utils

import "strings"

// HumanizeError converts low-level errors into actionable CLI messages.
func HumanizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "no such table"):
		return simpleError("Database not initialized.\nRun:\ndevgrep index")
	case strings.Contains(lower, "permission denied"):
		return simpleError("Permission denied while reading local files.\nCheck the path permissions or choose a narrower directory.")
	default:
		return err
	}
}

type simpleError string

func (e simpleError) Error() string {
	return string(e)
}
