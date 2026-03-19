package repo

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// validateRef checks that a git reference (hash, branch, tag, remote name)
// is not attempting to inject git command-line options.
// It rejects any ref that starts with '-' after trimming whitespace.
func validateRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty git reference")
	}
	if strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("invalid git reference %q: must not start with '-'", ref)
	}
	return ref, nil
}

// validatePath ensures that the resolved path stays within basePath,
// preventing path traversal attacks (e.g. "../../etc/passwd").
// It returns the resolved absolute path on success.
func validatePath(basePath, userPath string) (string, error) {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("unable to resolve base path: %w", err)
	}

	joined := filepath.Join(absBase, userPath)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("unable to resolve path: %w", err)
	}

	// Ensure the resolved path is within the base directory.
	// We append a separator to absBase to avoid false prefix matches
	// (e.g. /repo-evil matching /repo).
	if absJoined != absBase && !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes base directory", userPath)
	}

	return absJoined, nil
}

// sanitizeURL removes credentials (username:password) from a URL string
// for safe logging and error messages. Handles https://, ssh://, and
// user@host:path formats.
func sanitizeURL(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}

	// Try to parse as a standard URL (https://, ssh://, etc.)
	if strings.Contains(rawURL, "://") {
		u, err := url.Parse(rawURL)
		if err == nil && u.User != nil {
			u.User = nil
			return u.String()
		}
	}

	return rawURL
}

// sanitizeErrorMessage replaces any URLs containing credentials in an
// error string with their sanitized equivalents.
func sanitizeErrorMessage(msg string) string {
	result := msg
	offset := 0
	for {
		idx := strings.Index(result[offset:], "://")
		if idx < 0 {
			break
		}
		idx += offset

		// Walk backward to find the start of the scheme
		start := idx
		for start > 0 && isSchemeChar(result[start-1]) {
			start--
		}

		// Walk forward to find the end of the URL
		end := idx + 3
		for end < len(result) && !isURLTerminator(result[end]) {
			end++
		}

		rawURL := result[start:end]
		sanitized := sanitizeURL(rawURL)
		if sanitized != rawURL {
			result = result[:start] + sanitized + result[end:]
			offset = start + len(sanitized)
		} else {
			offset = end
		}
	}
	return result
}

func isSchemeChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.'
}

func isURLTerminator(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
		c == '"' || c == '\'' || c == '>'
}

// shellEscape escapes a string for safe inclusion in a shell script.
// It wraps the string in single quotes and escapes any embedded single quotes.
func shellEscape(s string) string {
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}
