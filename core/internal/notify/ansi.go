package notify

import (
	"regexp"
	"strings"
)

// ansiPattern matches ANSI escape sequences:
// - CSI sequences: \x1b[ ... (letter)
// - OSC sequences: \x1b] ... (\x07 or \x1b\\)
// - Other escapes: \x1b (single char)
var ansiPattern = regexp.MustCompile(`\x1b(?:\[[0-9;]*[a-zA-Z]|\][^\x07]*\x07|\[[0-9;]*m|.)`)

// StripANSI removes all ANSI escape sequences from a string.
func StripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// LastVisibleLine returns the last non-empty line of text after stripping
// ANSI codes and handling carriage returns.
func LastVisibleLine(s string) string {
	clean := StripANSI(s)

	// Handle carriage returns: keep only text after last \r on each line
	lines := strings.Split(clean, "\n")
	for i, line := range lines {
		if idx := strings.LastIndex(line, "\r"); idx >= 0 {
			lines[i] = line[idx+1:]
		}
	}

	// Find last non-empty line
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			return lines[i]
		}
	}
	return ""
}
